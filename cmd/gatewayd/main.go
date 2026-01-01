package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/admin"
	"sdk-microservices/internal/platform/authctx"
	"sdk-microservices/internal/platform/config"
	"sdk-microservices/internal/platform/health"
	"sdk-microservices/internal/platform/httpmw"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/otel"
	"sdk-microservices/internal/services/auth/jwt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	ctx := context.Background()

	log := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)).With(zap.String("service", "gateway"))
	defer func() { _ = log.Sync() }()

	shutdownOTEL, err := otel.Init(ctx, "gateway")
	if err != nil {
		log.Fatal("otel init", zap.Error(err))
	}
	defer func() { _ = shutdownOTEL(context.Background()) }()

	metricsH, shutdownMetrics, err := otel.InitMetricsPrometheus(ctx, "gateway")
	if err != nil {
		log.Fatal("metrics init", zap.Error(err))
	}
	defer func() { _ = shutdownMetrics(context.Background()) }()

	serving := atomic.Bool{}
	serving.Store(true)

	// Readiness dependencies.
	readyGraph := health.NewReadyGraph()
	readyGraph.Add("otel", health.CheckAlwaysReady())
	readyGraph.Add("metrics", health.CheckAlwaysReady())

	helloEndpoint := config.Getenv("HELLO_GRPC_ADDR", "localhost:9091")
	authEndpoint := config.Getenv("AUTH_GRPC_ADDR", "localhost:9092")

	// Downstream gRPC dials.
	dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second)
	defer dialCancel()

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	helloConn, err := grpc.DialContext(dialCtx, helloEndpoint, dialOpts...)
	if err != nil {
		log.Fatal("dial hello", zap.Error(err))
	}
	defer func() { _ = helloConn.Close() }()

	authConn, err := grpc.DialContext(dialCtx, authEndpoint, dialOpts...)
	if err != nil {
		log.Fatal("dial auth", zap.Error(err))
	}
	defer func() { _ = authConn.Close() }()

	// Local JWT validator to avoid per-request RPC fanout to authd.
	jwtSecret := config.Getenv("AUTH_JWT_SECRET", "dev-secret-change-me")
	jwtIssuer := config.Getenv("AUTH_JWT_ISSUER", "sdk-microservices")
	jwtSvc := jwt.New(jwtSecret, jwtIssuer)

	// gRPC-Gateway mux. We forward request_id and user_id into downstream metadata.
	mux := runtime.NewServeMux(
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			md := metadata.MD{}
			if rid := r.Header.Get("X-Request-Id"); rid != "" {
				md.Set("x-request-id", rid)
			}
			if uid, ok := authctx.UserID(ctx); ok {
				md.Set("x-user-id", uid)
			}
			return md
		}),
		runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
			// Make proxy errors visible in logs with trace_id/span_id.
			logging.WithTrace(ctx, log).Error("gateway proxy error",
				zap.String("path", r.URL.Path),
				zap.Error(err),
			)
			runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
		}),
	)

	if err := hellov1.RegisterHelloServiceHandlerClient(ctx, mux, hellov1.NewHelloServiceClient(helloConn)); err != nil {
		log.Fatal("register hello gateway", zap.Error(err))
	}
	if err := authv1.RegisterAuthServiceHandlerClient(ctx, mux, authv1.NewAuthServiceClient(authConn)); err != nil {
		log.Fatal("register auth gateway", zap.Error(err))
	}

	root := http.NewServeMux()
	root.Handle("/", mux)

	// Cheap liveness on the main listener (admin has /livez too).
	root.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Rate limiting: default 200 RPS / IP, burst 400 (tune per deployment).
	rl := httpmw.NewIPLimiter(
		rate.Limit(envFloat("GATEWAY_RATELIMIT_RPS", 200)),
		envInt("GATEWAY_RATELIMIT_BURST", 400),
		2*time.Minute,
	)

	// Compose the edge handler:
	// - request id early (so logs + metadata always have it)
	// - auth on all non-/v1/auth/* paths
	// - rate limit (after auth so 401s still count; flip if you prefer)
	// - security headers
	// - OTel + access logs
	h := httpmw.RequestID(root)
	h = authSkipper(jwtSvc, h)
	h = rl.Middleware(h)
	h = httpmw.SecurityHeaders(h)
	h = httpmw.Wrap("gateway", log, h)

	srv := &http.Server{
		Addr:              config.Getenv("GATEWAY_HTTP_ADDR", ":8080"),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	adminSrv, err := admin.Start(log, admin.Options{
		Addr:        config.Getenv("GATEWAY_ADMIN_ADDR", ":8081"),
		ServiceName: "gateway",
		Metrics:     metricsH,
		ReadyRoot:   readyGraph,
		ServingFn:   serving.Load,
	})
	if err != nil {
		log.Fatal("admin start", zap.Error(err))
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = adminSrv.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Info("gateway listening",
			zap.String("addr", srv.Addr),
			zap.String("hello_grpc", helloEndpoint),
			zap.String("auth_grpc", authEndpoint),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("gateway serve", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 2)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("shutting down gateway")
	serving.Store(false)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

// authSkipper enforces bearer auth everywhere except auth endpoints + health checks.
func authSkipper(jwtSvc *jwt.Service, next http.Handler) http.Handler {
	protected := httpmw.AuthBearer(jwtSvc, next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			next.ServeHTTP(w, r)
			return
		case stringsHasPrefix(r.URL.Path, "/v1/auth/"):
			next.ServeHTTP(w, r)
			return
		default:
			protected.ServeHTTP(w, r)
			return
		}
	})
}

func stringsHasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func envInt(k string, d int) int {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	return i
}

func envFloat(k string, d float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return d
	}
	return f
}

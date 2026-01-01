package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/admin"
	"sdk-microservices/internal/platform/health"
	"sdk-microservices/internal/platform/httpmw"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/metrics"
	"sdk-microservices/internal/platform/otel"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log, err := logging.New("gateway")
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()

	shutdownOTEL, err := otel.Init(ctx, "gateway")
	if err != nil {
		log.Fatal("otel init failed", zap.Error(err))
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = shutdownOTEL(shutdownCtx)
	}()

	// Metrics provider + /metrics handler (served on the admin port).
	metricsH, shutdownMetrics, err := otel.InitMetricsPrometheus(ctx, "gateway")
	if err != nil {
		log.Fatal("metrics init failed", zap.Error(err))
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = shutdownMetrics(shutdownCtx)
	}()

	httpm, err := metrics.NewHTTPServerMetrics("gateway")
	if err != nil {
		log.Fatal("http metrics init failed", zap.Error(err))
	}

	helloEndpoint := env("HELLO_GRPC_ENDPOINT", "localhost:50051")
	authEndpoint := env("AUTH_GRPC_ENDPOINT", "localhost:50052")

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

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

	mux := runtime.NewServeMux(
		runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
			log.Error("gateway proxy error",
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

	// Useful for cheap liveness checks on the main listener (separate from /readyz on the admin port).
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	handler := httpmw.Wrap("gateway", log, root)
	handler = httpm.Middleware(handler)

	srv := &http.Server{
		Addr:              env("GATEWAY_ADDR", ":8080"),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	var serving atomic.Bool
	serving.Store(true)

	readyGraph := &health.Node{
		Name: "gateway",
		Deps: []*health.Node{
			{Name: "hello", Check: health.GRPCHealthCheck(helloConn, "")},
			{Name: "auth", Check: health.GRPCHealthCheck(authConn, "")},
		},
	}

	adminSrv, err := admin.Start(log, admin.Options{
		Addr:        env("GATEWAY_ADMIN_ADDR", ":8081"),
		ServiceName: "gateway",
		Metrics:     metricsH,
		ReadyRoot:   readyGraph,
		ServingFn:   serving.Load,
	})
	if err != nil {
		log.Fatal("admin start failed", zap.Error(err))
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

	// Mark NOT_SERVING first, so /readyz flips quickly.
	serving.Store(false)

	// Drain HTTP requests.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	// Close downstream conns after HTTP drain.
	_ = helloConn.Close()
	_ = authConn.Close()
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

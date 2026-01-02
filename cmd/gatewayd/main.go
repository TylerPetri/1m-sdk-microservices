package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/authctx"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/httpmw"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	_ = boot.Run(context.Background(), boot.Options{
		ServiceName:     "gateway",
		AdminAddrEnv:    "GATEWAY_ADMIN_ADDR",
		ShutdownTimeout: 10 * time.Second,
	}, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
		log := deps.Log

		httpAddr := env("GATEWAY_HTTP_ADDR", ":8080")
		helloEndpoint := env("HELLO_GRPC_ADDR", "localhost:50051")
		authEndpoint := env("AUTH_GRPC_ADDR", "localhost:50052")

		helloConn, err := grpc.DialContext(ctx, helloEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return boot.Main{}, err
		}

		authConn, err := grpc.DialContext(ctx, authEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			_ = helloConn.Close()
			return boot.Main{}, err
		}

		mux := runtime.NewServeMux(
			runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
				md := metadata.MD{}
				if rid := r.Header.Get("x-request-id"); rid != "" {
					md.Append("x-request-id", rid)
				}
				if auth := r.Header.Get("authorization"); auth != "" {
					md.Append("authorization", auth)
				}
				return md
			}),
		)

		if err := hellov1.RegisterHelloServiceHandlerClient(ctx, mux, hellov1.NewHelloServiceClient(helloConn)); err != nil {
			_ = helloConn.Close()
			_ = authConn.Close()
			return boot.Main{}, err
		}
		if err := authv1.RegisterAuthServiceHandlerClient(ctx, mux, authv1.NewAuthServiceClient(authConn)); err != nil {
			_ = helloConn.Close()
			_ = authConn.Close()
			return boot.Main{}, err
		}

		root := http.NewServeMux()
		root.Handle("/", mux)
		root.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		// Default 200 rps / ip, burst 400.
		rl := httpmw.NewIPLimiter(
			rate.Limit(envFloat("GATEWAY_RATELIMIT_RPS", 200)),
			envInt("GATEWAY_RATELIMIT_BURST", 400),
			2*time.Minute,
		)

		edge := httpmw.EdgePolicy{
			ServiceName: "gateway",
			Timeout:     envDuration("GATEWAY_TIMEOUT", 30*time.Second),
			MaxInFlight: envInt("GATEWAY_MAX_INFLIGHT", 512),
			Leaf: httpmw.Chain{
				rl.Wrap,
				func(next http.Handler) http.Handler {
					return authctx.GatewayAuth("/v1/auth/", next)
				},
			},
		}

		h := httpmw.BuildEdgeHandler(log, edge, root)

		srv := &http.Server{
			Addr:              httpAddr,
			Handler:           h,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       90 * time.Second,
		}

		return boot.Main{
			Serve: func() error {
				log.Info("gateway listening",
					zap.String("addr", srv.Addr),
					zap.String("hello_grpc", helloEndpoint),
					zap.String("auth_grpc", authEndpoint),
				)
				return srv.ListenAndServe()
			},
			Shutdown: func(ctx context.Context) error {
				_ = helloConn.Close()
				_ = authConn.Close()
				return srv.Shutdown(ctx)
			},
		}, nil
	})
}

func env(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
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

func envDuration(k string, d time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	dur, err := time.ParseDuration(v)
	if err != nil {
		return d
	}
	return dur
}

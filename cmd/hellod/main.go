package main

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/grpcutil"
	hellosrv "sdk-microservices/internal/services/hello/server"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	grpc_health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	_ = boot.Run(context.Background(), boot.Options{
		ServiceName: "hello",
		AdminAddrEnv: "HELLO_ADMIN_ADDR",
		ShutdownTimeout: 10 * time.Second,
	}, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
		log := deps.Log
		addr := env("HELLO_ADDR", ":50051")

		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return boot.Main{}, err
		}

		gs := grpc.NewServer(grpcutil.ServerOptionsWithNameAndLimits("hello", log, grpcutil.Limits{
			DefaultTimeout: envDuration("HELLO_RPC_TIMEOUT", 10*time.Second),
			MaxInFlight:    envInt("HELLO_MAX_INFLIGHT", 256),
		})...)

		hellov1.RegisterHelloServiceServer(gs, &hellosrv.Server{})

		hs := grpc_health.NewServer()
		hs.SetServingStatus("hello.v1.HelloService", healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(gs, hs)

		return boot.Main{
			Serve: func() error {
				log.Info("hellod listening", zap.String("addr", addr))
				return gs.Serve(lis)
			},
			Shutdown: func(ctx context.Context) error {
				hs.SetServingStatus("hello.v1.HelloService", healthpb.HealthCheckResponse_NOT_SERVING)
				done := make(chan struct{})
				go func() {
					gs.GracefulStop()
					close(done)
				}()
				select {
				case <-done:
				case <-ctx.Done():
					gs.Stop()
				}
				_ = lis.Close()
				return nil
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

package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/grpcutil"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/otel"
	hellosrv "sdk-microservices/internal/services/hello/server"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	log, err := logging.New("hello")
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	shutdownOTEL, err := otel.Init(context.Background(), "hello")
	if err != nil {
		log.Fatal("otel init failed", zap.Error(err))
	}
	defer func() { _ = shutdownOTEL(context.Background()) }()

	addr := env("HELLO_ADDR", ":50051")

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.Error(err))
	}

	gs := grpc.NewServer(grpcutil.ServerOptionsWithName("hello", log)...)
	hs := health.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hellov1.RegisterHelloServiceServer(gs, &hellosrv.Server{})

	log.Info("hello service listening", zap.String("addr", addr))

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Info("shutting down hello service")
		gs.GracefulStop()
	}()

	if err := gs.Serve(lis); err != nil {
		log.Fatal("serve failed", zap.Error(err))
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

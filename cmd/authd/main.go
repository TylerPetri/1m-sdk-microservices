package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	"sdk-microservices/internal/db"
	"sdk-microservices/internal/platform/admin"
	"sdk-microservices/internal/platform/config"
	"sdk-microservices/internal/platform/grpcutil"
	"sdk-microservices/internal/platform/health"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/otel"
	"sdk-microservices/internal/services/auth/jwt"
	authsrv "sdk-microservices/internal/services/auth/server"
	"sdk-microservices/internal/services/auth/store"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	ctx := context.Background()

	log, err := logging.New("auth")
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	shutdownOTEL, err := otel.Init(ctx, "auth")
	if err != nil {
		log.Fatal("otel init failed", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownOTEL(shutdownCtx)
	}()

	metricsH, shutdownMetrics, err := otel.InitMetricsPrometheus(ctx, "auth")
	if err != nil {
		log.Fatal("metrics init failed", zap.Error(err))
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = shutdownMetrics(shutdownCtx)
	}()

	addr := config.Getenv("AUTH_ADDR", ":50052")
	adminAddr := config.Getenv("AUTH_ADMIN_ADDR", ":8081")
	dsn := config.Getenv("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/auth?sslmode=disable")
	jwtSecret := config.Getenv("AUTH_JWT_SECRET", "dev-secret-change-me")
	issuer := config.Getenv("AUTH_JWT_ISSUER", "sdk-microservices")

	dbConn, err := db.NewPool(ctx, dsn, db.Options{})
	if err != nil {
		log.Fatal("db connect failed", zap.Error(err))
	}
	defer func() { dbConn.Close() }()

	st := store.New(dbConn)
	jwtSvc := jwt.New(jwtSecret, issuer)

	srv := authsrv.New(log, st, jwtSvc, authsrv.Options{
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	})

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("listen failed", zap.Error(err))
	}

	gs := grpc.NewServer(grpcutil.ServerOptionsWithNameAndLimits("auth", log, grpcutil.Limits{
		DefaultTimeout: envDuration("AUTH_RPC_TIMEOUT", 10*time.Second),
		MaxInFlight:    envInt("AUTH_MAX_INFLIGHT", 2048),
	})...)

	hs := healthgrpc.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	authv1.RegisterAuthServiceServer(gs, srv)

	var serving atomic.Bool
	serving.Store(true)

	readyGraph := &health.Node{
		Name: "auth",
		Deps: []*health.Node{
			{Name: "postgres", Check: health.SQLPing(dbConn)},
		},
	}

	adminSrv, err := admin.Start(log, admin.Options{
		Addr:        adminAddr,
		ServiceName: "auth",
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

	log.Info("auth service listening", zap.String("addr", addr))

	// Shutdown ordering: mark NOT_SERVING → drain → stop.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		log.Info("shutting down auth service")

		serving.Store(false)
		hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		done := make(chan struct{})
		go func() {
			gs.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Warn("graceful stop timed out; forcing stop")
			gs.Stop()
		}

		dbConn.Close()
	}()

	if err := gs.Serve(lis); err != nil {
		log.Fatal("serve failed", zap.Error(err))
	}
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

func envDuration(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			return dur
		}
	}
	return d
}

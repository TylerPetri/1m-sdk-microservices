package main

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	"sdk-microservices/internal/db"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/grpcutil"
	"sdk-microservices/internal/services/auth/jwt"
	authsrv "sdk-microservices/internal/services/auth/server"
	"sdk-microservices/internal/services/auth/store"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	grpc_health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	_ = boot.Run(context.Background(), boot.Options{
		ServiceName:     "auth",
		AdminAddrEnv:    "AUTH_ADMIN_ADDR",
		ShutdownTimeout: 10 * time.Second,
	}, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
		log := deps.Log

		addr := env("AUTH_ADDR", ":50052")
		dsn := env("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/auth?sslmode=disable")
		jwtSecret := env("AUTH_JWT_SECRET", "dev-secret-change-me")
		issuer := env("AUTH_JWT_ISSUER", "sdk-microservices")

		pool, err := db.NewPool(ctx, dsn, db.Options{
			MaxConns:          int32(envInt("AUTH_DB_MAX_CONNS", 20)),
			MinConns:          int32(envInt("AUTH_DB_MIN_CONNS", 2)),
			MaxConnLifetime:   envDuration("AUTH_DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   envDuration("AUTH_DB_MAX_CONN_IDLE", 5*time.Minute),
			HealthCheckPeriod: envDuration("AUTH_DB_HEALTHCHECK", 30*time.Second),
		})
		if err != nil {
			return boot.Main{}, err
		}

		st := store.New(pool)
		jwtSvc := jwt.New(jwtSecret, issuer)

		srv := authsrv.New(log, st, jwtSvc, authsrv.Options{
			AccessTTL:  envDuration("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: envDuration("AUTH_REFRESH_TTL", 7*24*time.Hour),
		})

		lis, err := net.Listen("tcp", addr)
		if err != nil {
			pool.Close()
			return boot.Main{}, err
		}

		gs := grpc.NewServer(grpcutil.ServerOptionsWithNameAndLimits("auth", log, grpcutil.Limits{
			DefaultTimeout: envDuration("AUTH_RPC_TIMEOUT", 10*time.Second),
			MaxInFlight:    envInt("AUTH_MAX_INFLIGHT", 256),
		})...)

		authv1.RegisterAuthServiceServer(gs, srv)

		hs := grpc_health.NewServer()
		hs.SetServingStatus("auth.v1.AuthService", healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(gs, hs)

		return boot.Main{
			Serve: func() error {
				log.Info("authd listening", zap.String("addr", addr))
				return gs.Serve(lis)
			},
			Shutdown: func(ctx context.Context) error {
				hs.SetServingStatus("auth.v1.AuthService", healthpb.HealthCheckResponse_NOT_SERVING)
				// GracefulStop does not take context; emulate with a deadline + Stop fallback.
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
				pool.Close()
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

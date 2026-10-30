package main

import (
	"context"
	"net"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	"sdk-microservices/internal/db"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/config"
	"sdk-microservices/internal/platform/grpcutil"
	authcfg "sdk-microservices/internal/services/auth"
	"sdk-microservices/internal/services/auth/jwt"
	authsrv "sdk-microservices/internal/services/auth/server"
	"sdk-microservices/internal/services/auth/store"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	grpc_health "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	cfg := config.MustLoad[authcfg.Config]()

	_ = boot.Run(context.Background(), boot.Options{
		ServiceName:     "auth",
		AdminAddr:       cfg.Admin.Addr,
		ShutdownTimeout: 10 * time.Second,
	}, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
		log := deps.Log
		config.LogEffective(log, cfg)

		lis, err := net.Listen("tcp", cfg.GRPC.Addr)
		if err != nil {
			return boot.Main{}, err
		}

		pool, err := db.NewPool(ctx, cfg.DB.DSN, db.Options{
			MaxConns:          int32(cfg.DB.Pool.MaxConns),
			MinConns:          int32(cfg.DB.Pool.MinConns),
			MaxConnLifetime:   cfg.DB.Pool.MaxConnLifetime,
			MaxConnIdleTime:   cfg.DB.Pool.MaxConnIdleTime,
			HealthCheckPeriod: cfg.DB.Pool.HealthCheckPeriod,
		})
		if err != nil {
			return boot.Main{}, err
		}

		st := store.New(pool)
		jwtSvc := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer)

		srv := authsrv.New(log, st, jwtSvc, authsrv.Options{
			AccessTTL:  cfg.Tokens.AccessTTL,
			RefreshTTL: cfg.Tokens.RefreshTTL,
		})

		gs := grpc.NewServer(grpcutil.ServerOptionsWithNameAndLimits("auth", log, grpcutil.Limits{
			DefaultTimeout: cfg.GRPC.DefaultTimeout,
			MaxInFlight:    cfg.GRPC.MaxInFlight,
		})...)

		authv1.RegisterAuthServiceServer(gs, srv)

		hs := grpc_health.NewServer()
		hs.SetServingStatus("auth.v1.AuthService", healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(gs, hs)

		return boot.Main{
			Serve: func() error {
				log.Info("authd listening", zap.String("addr", cfg.GRPC.Addr))
				return gs.Serve(lis)
			},
			Shutdown: func(ctx context.Context) error {
				done := make(chan struct{})
				go func() {
					gs.GracefulStop()
					close(done)
				}()
				select {
				case <-done:
				case <-ctx.Done():
					gs.Stop()
					pool.Close()
					return ctx.Err()
				}
				pool.Close()
				return nil
			},
		}, nil
	})
}

package main

import (
    "context"
    "net"
    "time"

    hellov1 "sdk-microservices/gen/api/proto/hello/v1"
    "sdk-microservices/internal/platform/boot"
    "sdk-microservices/internal/platform/config"
    "sdk-microservices/internal/platform/grpcutil"
    hellocfg "sdk-microservices/internal/services/hello"
    hellosrv "sdk-microservices/internal/services/hello/server"

    "go.uber.org/zap"
    "google.golang.org/grpc"
    grpc_health "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
    cfg := config.MustLoad[hellocfg.Config]()

    _ = boot.Run(context.Background(), boot.Options{
        ServiceName:     "hello",
        AdminAddr:       cfg.Admin.Addr,
        ShutdownTimeout: 10 * time.Second,
    }, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
        log := deps.Log
        config.LogEffective(log, cfg)

        lis, err := net.Listen("tcp", cfg.GRPC.Addr)
        if err != nil {
            return boot.Main{}, err
        }

        srv := hellosrv.New(log)

        gs := grpc.NewServer(grpcutil.ServerOptionsWithNameAndLimits("hello", log, grpcutil.Limits{
            DefaultTimeout: cfg.GRPC.DefaultTimeout,
            MaxInFlight:    cfg.GRPC.MaxInFlight,
        })...)

        hellov1.RegisterHelloServiceServer(gs, srv)

        hs := grpc_health.NewServer()
        hs.SetServingStatus("hello.v1.HelloService", healthpb.HealthCheckResponse_SERVING)
        healthpb.RegisterHealthServer(gs, hs)

        return boot.Main{
            Serve: func() error {
                log.Info("hellod listening", zap.String("addr", cfg.GRPC.Addr))
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
                    return nil
                case <-ctx.Done():
                    gs.Stop()
                    return ctx.Err()
                }
            },
        }, nil
    })
}

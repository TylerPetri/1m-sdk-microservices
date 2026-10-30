package main

import (
	"context"
	"net/http"
	"time"

	authv1 "sdk-microservices/gen/api/proto/auth/v1"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/config"
	"sdk-microservices/internal/platform/httpmw"
	gatewaycfg "sdk-microservices/internal/services/gateway"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	cfg := config.MustLoad[gatewaycfg.Config]()

	_ = boot.Run(context.Background(), boot.Options{
		ServiceName:     "gateway",
		AdminAddr:       cfg.Admin.Addr,
		ShutdownTimeout: 10 * time.Second,
	}, func(ctx context.Context, deps boot.Deps) (boot.Main, error) {
		log := deps.Log
		config.LogEffective(log, cfg)

		helloConn, err := grpc.DialContext(ctx, cfg.Upstreams.HelloGRPCAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return boot.Main{}, err
		}

		authConn, err := grpc.DialContext(ctx, cfg.Upstreams.AuthGRPCAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			_ = helloConn.Close()
			return boot.Main{}, err
		}

		mux := runtime.NewServeMux(
			runtime.WithMetadata(func(ctx context.Context, req *http.Request) metadata.MD {
				// propagate gateway request-id to gRPC for log correlation
				if rid := req.Header.Get(httpmw.HeaderRequestID); rid != "" {
					return metadata.Pairs(httpmw.HeaderRequestID, rid)
				}
				return nil
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
		root.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

		h := gatewaycfg.BuildEdgeMiddleware(log, cfg, root)

		srv := &http.Server{
			Addr:    cfg.HTTP.Addr,
			Handler: h,
		}

		return boot.Main{
			Serve: func() error {
				log.Info("gateway listening",
					zap.String("addr", srv.Addr),
					zap.String("hello_grpc", cfg.Upstreams.HelloGRPCAddr),
					zap.String("auth_grpc", cfg.Upstreams.AuthGRPCAddr),
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

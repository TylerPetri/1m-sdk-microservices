package grpcutil

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// DialReady preserves startup dependency readiness with the current gRPC client API.
// The caller owns the context deadline and the returned connection.
func DialReady(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if state == connectivity.Shutdown {
			_ = conn.Close()
			return nil, fmt.Errorf("gRPC connection shut down before ready")
		}
		conn.Connect()
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, ctx.Err()
		}
	}
}

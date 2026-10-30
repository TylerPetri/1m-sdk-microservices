package grpcutil

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryTimeout applies a default timeout to unary RPCs that do not already
// have a deadline.
func UnaryTimeout(d time.Duration) grpc.UnaryServerInterceptor {
	if d <= 0 {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); ok {
			return handler(ctx, req)
		}
		c, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(c, req)
	}
}

// UnaryInFlightLimit bounds concurrent in-flight unary RPCs.
// If the limit is reached, it returns ResourceExhausted.
func UnaryInFlightLimit(max int) grpc.UnaryServerInterceptor {
	if max <= 0 {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	sem := make(chan struct{}, max)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			return handler(ctx, req)
		default:
			return nil, status.Error(codes.ResourceExhausted, "too many in-flight requests")
		}
	}
}

// StreamInFlightLimit bounds concurrent in-flight streaming RPCs.
// If the limit is reached, it returns ResourceExhausted.
func StreamInFlightLimit(max int) grpc.StreamServerInterceptor {
	if max <= 0 {
		return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}

	sem := make(chan struct{}, max)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			return handler(srv, ss)
		default:
			return status.Error(codes.ResourceExhausted, "too many in-flight streams")
		}
	}
}

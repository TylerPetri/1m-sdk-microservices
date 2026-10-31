package grpcutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestUnaryTimeoutCancellation(t *testing.T) {
	_, err := UnaryTimeout(20*time.Millisecond)(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cause=%v", err)
	}
}

func TestUnaryTimeoutPreservesCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	want, _ := ctx.Deadline()
	_, err := UnaryTimeout(time.Minute)(ctx, nil, &grpc.UnaryServerInfo{}, func(got context.Context, _ any) (any, error) {
		deadline, ok := got.Deadline()
		if !ok || deadline != want {
			t.Error("caller deadline changed")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

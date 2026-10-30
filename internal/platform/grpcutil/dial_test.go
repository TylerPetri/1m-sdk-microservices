package grpcutil

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func TestDialReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := DialReady(ctx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.GetState() != connectivity.Ready {
		t.Fatal("returned before transport readiness")
	}
}

func TestDialReadyCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := DialReady(ctx, "127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if conn != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("conn=%v err=%v", conn, err)
	}
}

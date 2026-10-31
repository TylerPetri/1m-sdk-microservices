package grpcutil

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	hellov1 "sdk-microservices/gen/api/proto/hello/v1"
)

// Test-only implementation of the existing API, not a deployed fault endpoint.
type waitingHello struct {
	hellov1.UnimplementedHelloServiceServer
	entered  chan struct{}
	observed chan error
}

func (s *waitingHello) Hello(ctx context.Context, _ *hellov1.HelloRequest) (*hellov1.HelloResponse, error) {
	close(s.entered)
	<-ctx.Done()
	s.observed <- ctx.Err()
	return nil, ctx.Err()
}

func waitingGateway(t *testing.T, timeout time.Duration) (*httptest.Server, <-chan struct{}, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	service := &waitingHello{entered: make(chan struct{}), observed: make(chan error, 1)}
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(UnaryTimeout(timeout)))
	hellov1.RegisterHelloServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := DialReady(ctx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	mux := runtime.NewServeMux()
	if err := hellov1.RegisterHelloServiceHandlerClient(context.Background(), mux, hellov1.NewHelloServiceClient(conn)); err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(mux)
	gateway.Client().Timeout = 5 * time.Second
	t.Cleanup(gateway.Close)
	return gateway, service.entered, service.observed
}

func TestRPCDeadlineMapsToHTTP504(t *testing.T) {
	gateway, _, observed := waitingGateway(t, 20*time.Millisecond)
	response, err := gateway.Client().Get(gateway.URL + "/v1/hello/deadline")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status=%d", response.StatusCode)
	}
	select {
	case cause := <-observed:
		if !errors.Is(cause, context.DeadlineExceeded) {
			t.Fatalf("server cause=%v", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RPC context deadline was not observed")
	}
}

func TestHTTPClientCancellationReachesRPC(t *testing.T) {
	gateway, entered, observed := waitingGateway(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/v1/hello/cancel", nil)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		response, err := gateway.Client().Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("RPC handler did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client cause=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP caller did not return")
	}
	select {
	case cause := <-observed:
		if !errors.Is(cause, context.Canceled) {
			t.Fatalf("RPC cause=%v", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("caller cancellation did not reach RPC handler")
	}
}

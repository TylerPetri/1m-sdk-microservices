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
	observed chan error
}

func (s *waitingHello) Hello(ctx context.Context, _ *hellov1.HelloRequest) (*hellov1.HelloResponse, error) {
	<-ctx.Done()
	s.observed <- ctx.Err()
	return nil, ctx.Err()
}

func TestRPCDeadlineMapsToHTTP504(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	observed := make(chan error, 1)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(UnaryTimeout(20 * time.Millisecond)))
	hellov1.RegisterHelloServiceServer(server, &waitingHello{observed: observed})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := DialReady(ctx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	mux := runtime.NewServeMux()
	if err := hellov1.RegisterHelloServiceHandlerClient(ctx, mux, hellov1.NewHelloServiceClient(conn)); err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(mux)
	defer gateway.Close()
	request, err := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/v1/hello/deadline", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := gateway.Client().Do(request)
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
	case <-ctx.Done():
		t.Fatal("RPC context deadline was not observed")
	}
}

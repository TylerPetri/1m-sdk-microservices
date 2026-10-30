package server

import (
	"context"
	"fmt"

	hellov1 "sdk-microservices/gen/api/proto/hello/v1"

	"go.uber.org/zap"
)

type Server struct {
	hellov1.UnimplementedHelloServiceServer
	log *zap.Logger
}

// New constructs a Hello gRPC server.
func New(log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{log: log}
}

func (s *Server) Hello(ctx context.Context, req *hellov1.HelloRequest) (*hellov1.HelloResponse, error) {
	name := req.GetName()
	if name == "" {
		name = "world"
	}
	return &hellov1.HelloResponse{
		Message: fmt.Sprintf("hello, %s", name),
	}, nil
}

#!/usr/bin/env bash
set -euo pipefail
# Install into Go's bin directory; add $(go env GOPATH)/bin to PATH.
go install github.com/bufbuild/buf/cmd/buf@v1.47.2
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.3

# syntax=docker/dockerfile:1

FROM golang:1.24.9 AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# ---- build probe once
FROM golang:1.24.9 AS probes
RUN GOBIN=/out go install github.com/grpc-ecosystem/grpc-health-probe@latest

# ---- gateway
FROM base AS gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gatewayd ./cmd/gatewayd

FROM gcr.io/distroless/base-debian12 AS gatewayd-run
COPY --from=gateway /out/gatewayd /gatewayd
COPY --from=probes /out/grpc-health-probe /grpc_health_probe
ENTRYPOINT ["/gatewayd"]

# ---- hello
FROM base AS hello
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/hellod ./cmd/hellod

FROM gcr.io/distroless/base-debian12 AS hellod-run
COPY --from=hello /out/hellod /hellod
COPY --from=probes /out/grpc-health-probe /grpc_health_probe
ENTRYPOINT ["/hellod"]

# ---- auth
FROM base AS auth
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/authd ./cmd/authd

FROM gcr.io/distroless/base-debian12 AS authd-run
COPY --from=auth /out/authd /authd
COPY --from=probes /out/grpc-health-probe /grpc_health_probe
ENTRYPOINT ["/authd"]

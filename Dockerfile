# syntax=docker/dockerfile:1

FROM golang:1.24.9 AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    go mod download
COPY . .

# ---- build probe once
FROM golang:1.24.9 AS probes
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    CGO_ENABLED=0 GOBIN=/out go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.38

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

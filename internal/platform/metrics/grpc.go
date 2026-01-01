package metrics

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// GRPCServerMetrics provides low-cardinality gRPC server metrics.
type GRPCServerMetrics struct {
	service string

	inflight metric.Int64UpDownCounter
	errors   metric.Int64Counter
	latency  metric.Float64Histogram
}

func NewGRPCServerMetrics(service string) (*GRPCServerMetrics, error) {
	m := otel.Meter("sdk-microservices/" + service)

	inflight, err := m.Int64UpDownCounter(
		"rpc.server.inflight",
		metric.WithDescription("In-flight RPCs"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	errors, err := m.Int64Counter(
		"rpc.server.errors",
		metric.WithDescription("RPC errors (non-OK)"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	latency, err := m.Float64Histogram(
		"rpc.server.duration",
		metric.WithDescription("RPC server duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCServerMetrics{
		service:  service,
		inflight: inflight,
		errors:   errors,
		latency:  latency,
	}, nil
}

func (g *GRPCServerMetrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if g == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		method := lowCardMethod(info.FullMethod)
		base := []attribute.KeyValue{
			attribute.String("service.name", g.service),
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", method),
		}

		g.inflight.Add(ctx, 1, metric.WithAttributes(base...))
		defer g.inflight.Add(ctx, -1, metric.WithAttributes(base...))

		resp, err := handler(ctx, req)

		st := status.Convert(err)
		code := st.Code().String()

		attrs := append(base, attribute.String("rpc.code", code))

		g.latency.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		if st.Code().String() != "OK" {
			g.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
		}

		return resp, err
	}
}

func (g *GRPCServerMetrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	if g == nil {
		return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()

		method := lowCardMethod(info.FullMethod)
		base := []attribute.KeyValue{
			attribute.String("service.name", g.service),
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", method),
			attribute.Bool("rpc.stream", true),
		}

		g.inflight.Add(ctx, 1, metric.WithAttributes(base...))
		defer g.inflight.Add(ctx, -1, metric.WithAttributes(base...))

		err := handler(srv, ss)

		st := status.Convert(err)
		code := st.Code().String()
		attrs := append(base, attribute.String("rpc.code", code))

		g.latency.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		if st.Code().String() != "OK" {
			g.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
		}

		return err
	}
}

// lowCardMethod turns "/pkg.Service/Method" into "Service/Method" to keep labels sane.
func lowCardMethod(full string) string {
	full = strings.TrimPrefix(full, "/")
	parts := strings.Split(full, "/")
	if len(parts) != 2 {
		return full
	}
	svc := parts[0]
	if dot := strings.LastIndex(svc, "."); dot >= 0 {
		svc = svc[dot+1:]
	}
	return svc + "/" + parts[1]
}

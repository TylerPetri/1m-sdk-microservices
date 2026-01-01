package grpcutil

import (
	"context"
	"time"

	"sdk-microservices/internal/platform/authctx"
	"sdk-microservices/internal/platform/logging"
	"sdk-microservices/internal/platform/metrics"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ServerOptionsWith adds keepalives + OTel tracing/metrics + structured request logging.
// Deprecated: prefer ServerOptionsWithName to set an explicit service name for metrics labels.
func ServerOptionsWith(log *zap.Logger) []grpc.ServerOption {
	return ServerOptionsWithName("unknown", log)
}

// ServerOptionsWithName adds keepalives + OTel tracing/metrics + structured request logging.
func ServerOptionsWithName(service string, log *zap.Logger) []grpc.ServerOption {
	return ServerOptionsWithNameAndLimits(service, log, Limits{})
}

// Limits configures default timeouts + backpressure for gRPC servers.
type Limits struct {
	// DefaultTimeout is applied when the incoming context has no deadline.
	DefaultTimeout time.Duration
	// MaxInFlight bounds concurrent unary requests and streams.
	MaxInFlight int
}

// ServerOptionsWithNameAndLimits adds keepalives + OTel tracing/metrics + structured request logging,
// plus optional timeout/backpressure limits.
func ServerOptionsWithNameAndLimits(service string, log *zap.Logger, lim Limits) []grpc.ServerOption {
	opts := ServerOptions()

	// OTel tracing instrumentation (newer contrib uses StatsHandler, not interceptors).
	opts = append(opts,
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	var mu grpc.UnaryServerInterceptor
	var ms grpc.StreamServerInterceptor
	if m, err := metrics.NewGRPCServerMetrics(service); err == nil {
		mu = m.UnaryServerInterceptor()
		ms = m.StreamServerInterceptor()
	} else if log != nil {
		log.Warn("grpc metrics disabled (init failed)", zap.Error(err))
	}

	// Keep interceptors for limits + logging (best place to measure duration + map status codes).
	var unary []grpc.UnaryServerInterceptor
	// Apply backpressure/timeouts as early as possible.
	if lim.MaxInFlight > 0 {
		unary = append(unary, UnaryInFlightLimit(lim.MaxInFlight))
	}
	if lim.DefaultTimeout > 0 {
		unary = append(unary, UnaryTimeout(lim.DefaultTimeout))
	}
	if mu != nil {
		unary = append(unary, mu)
	}
	unary = append(unary, requestLogUnary(log))

	var stream []grpc.StreamServerInterceptor
	if lim.MaxInFlight > 0 {
		stream = append(stream, StreamInFlightLimit(lim.MaxInFlight))
	}
	if ms != nil {
		stream = append(stream, ms)
	}
	stream = append(stream, requestLogStream(log))

	opts = append(opts,
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	)

	return opts
}

func requestLogUnary(base *zap.Logger) grpc.UnaryServerInterceptor {
	if base == nil {
		base = zap.NewNop()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		lg := logging.WithTrace(ctx, base).With(
			zap.String("rpc.system", "grpc"),
			zap.String("rpc.method", info.FullMethod),
		)

		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			lg = lg.With(zap.String("client.addr", p.Addr.String()))
		}
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if rid := first(md, "x-request-id"); rid != "" {
				lg = lg.With(zap.String("request_id", rid))
			}
			if uid := first(md, "x-user-id"); uid != "" {
				lg = lg.With(zap.String("user_id", uid))
				ctx = authctx.WithUserID(ctx, uid)
			}
			if ua := first(md, "user-agent"); ua != "" {
				lg = lg.With(zap.String("user_agent", ua))
			}
		}

		ctx = logging.With(ctx, lg)
		resp, err := handler(ctx, req)

		st := status.Convert(err)
		lg.Info("rpc",
			zap.String("rpc.code", st.Code().String()),
			zap.Duration("duration", time.Since(start)),
		)

		return resp, err
	}
}

func requestLogStream(base *zap.Logger) grpc.StreamServerInterceptor {
	if base == nil {
		base = zap.NewNop()
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()

		lg := logging.WithTrace(ctx, base).With(
			zap.String("rpc.system", "grpc"),
			zap.String("rpc.method", info.FullMethod),
			zap.Bool("rpc.stream", true),
		)

		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			lg = lg.With(zap.String("client.addr", p.Addr.String()))
		}
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if rid := first(md, "x-request-id"); rid != "" {
				lg = lg.With(zap.String("request_id", rid))
			}
			if ua := first(md, "user-agent"); ua != "" {
				lg = lg.With(zap.String("user_agent", ua))
			}
		}

		wrapped := &wrappedStream{ServerStream: ss, ctx: logging.With(ctx, lg)}
		err := handler(srv, wrapped)

		st := status.Convert(err)
		lg.Info("rpc",
			zap.String("rpc.code", st.Code().String()),
			zap.Duration("duration", time.Since(start)),
		)

		return err
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func first(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

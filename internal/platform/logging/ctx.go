package logging

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ctxKey struct{}

func With(ctx context.Context, l *zap.Logger) context.Context {
	if l == nil {
		l = zap.NewNop()
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

func From(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if ctx == nil {
		if fallback != nil {
			return fallback
		}
		return zap.NewNop()
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if l, ok := v.(*zap.Logger); ok && l != nil {
			return l
		}
	}
	if fallback != nil {
		return fallback
	}
	return zap.NewNop()
}

func WithTrace(ctx context.Context, l *zap.Logger) *zap.Logger {
	if l == nil {
		l = zap.NewNop()
	}
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return l
	}
	return l.With(
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	)
}

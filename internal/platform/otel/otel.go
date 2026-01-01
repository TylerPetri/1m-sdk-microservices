package otel

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	stdouttrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// ShutdownFn shuts down the OTEL providers.
type ShutdownFn func(context.Context) error

// Init configures global OpenTelemetry tracing.
//
// Behavior:
//   - If OTEL_EXPORTER_OTLP_ENDPOINT is set, exports traces to that endpoint.
//   - Otherwise falls back to a dev-friendly stdout exporter.
//
// Supported env vars (subset of OTEL standard):
//   - OTEL_EXPORTER_OTLP_ENDPOINT
//   - OTEL_EXPORTER_OTLP_PROTOCOL ("grpc" or "http/protobuf")
//   - OTEL_EXPORTER_OTLP_INSECURE ("true"/"false") for grpc
//   - OTEL_RESOURCE_ATTRIBUTES (standard)
//   - OTEL_TRACES_SAMPLER (standard, handled by SDK)
//   - OTEL_TRACES_SAMPLER_ARG (standard, handled by SDK)
func Init(ctx context.Context, serviceName string, extraAttrs ...attribute.KeyValue) (ShutdownFn, error) {
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
		resource.WithAttributes(extraAttrs...),
	)
	if err != nil {
		return nil, err
	}

	exp, shutdownExp, err := newTraceExporter(ctx)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// Respect OTEL_TRACES_SAMPLER env automatically.
		sdktrace.WithBatcher(exp,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithMaxExportBatchSize(512),
		),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		var errs []error
		if shutdownExp != nil {
			if err := shutdownExp(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}, nil
}

func newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		exp, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		return exp, func(context.Context) error { return nil }, err
	}

	proto := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	switch strings.ToLower(proto) {
	case "", "grpc":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(endpoint),
		}
		if strings.EqualFold(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"), "true") {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		client := otlptracegrpc.NewClient(opts...)
		exp, err := otlptrace.New(ctx, client)
		if err != nil {
			return nil, nil, err
		}
		return exp, exp.Shutdown, nil
	case "http/protobuf", "http":
		client := otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(endpoint),
		)
		exp, err := otlptrace.New(ctx, client)
		if err != nil {
			return nil, nil, err
		}
		return exp, exp.Shutdown, nil
	default:
		return nil, nil, errors.New("unsupported OTEL_EXPORTER_OTLP_PROTOCOL: " + proto)
	}
}

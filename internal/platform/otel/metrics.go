package otel

import (
	"context"
	"net/http"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitMetricsPrometheus wires an OTEL MeterProvider backed by a Prometheus scrape endpoint.
// It returns the /metrics handler and a shutdown function.
func InitMetricsPrometheus(
	ctx context.Context,
	serviceName string,
	extraAttrs ...attribute.KeyValue,
) (http.Handler, func(context.Context) error, error) {

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithAttributes(extraAttrs...),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create a dedicated registry per service (clean separation, avoids global default registry issues).
	reg := prom.NewRegistry()

	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // cpu, mem, fds, etc
		collectors.NewGoCollector(),                                       // gc, goroutines, heap, etc
	)

	// Exporter registers metrics into the provided Prometheus registry.
	exp, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exp),
	)
	otel.SetMeterProvider(mp)
	if err := runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(10 * time.Second),
	); err != nil {
		return nil, nil, err
	}

	// Expose /metrics from that registry.
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	return h, mp.Shutdown, nil
}

package metrics

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPServerMetrics provides low-cardinality HTTP server metrics for a gateway-style HTTP server.
type HTTPServerMetrics struct {
	service string

	inflight metric.Int64UpDownCounter
	errors   metric.Int64Counter
	latency  metric.Float64Histogram
}

func NewHTTPServerMetrics(service string) (*HTTPServerMetrics, error) {
	m := otel.Meter("sdk-microservices/" + service)

	inflight, err := m.Int64UpDownCounter(
		"http.server.inflight",
		metric.WithDescription("In-flight HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	errors, err := m.Int64Counter(
		"http.server.errors",
		metric.WithDescription("HTTP 5xx responses"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}
	latency, err := m.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("HTTP server duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPServerMetrics{
		service:  service,
		inflight: inflight,
		errors:   errors,
		latency:  latency,
	}, nil
}

func (h *HTTPServerMetrics) Middleware(next http.Handler) http.Handler {
	if h == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}

		attrs := []attribute.KeyValue{
			attribute.String("service.name", h.service),
			attribute.String("http.method", r.Method),
		}

		h.inflight.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		defer h.inflight.Add(r.Context(), -1, metric.WithAttributes(attrs...))

		next.ServeHTTP(sw, r)

		codeStr := strconv.Itoa(sw.status)
		attrs = append(attrs, attribute.String("http.status_code", codeStr))

		dur := time.Since(start).Seconds()
		h.latency.Record(r.Context(), dur, metric.WithAttributes(attrs...))

		if sw.status >= 500 {
			h.errors.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		}
	})
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Ensure we satisfy interfaces at compile time.
var _ http.ResponseWriter = (*statusCapturingResponseWriter)(nil)

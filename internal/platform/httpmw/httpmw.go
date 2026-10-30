package httpmw

import (
	"net/http"
	"time"

	"sdk-microservices/internal/platform/logging"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// Wrap adds OpenTelemetry spans + structured access logging.
func Wrap(service string, log *zap.Logger, next http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}

	accessLog := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &respWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		lg := logging.WithTrace(r.Context(), log).With(
			zap.String("http.method", r.Method),
			zap.String("http.path", r.URL.Path),
			zap.Int("http.status", sw.status),
			zap.Duration("duration", time.Since(start)),
		)

		if rid := r.Header.Get("x-request-id"); rid != "" {
			lg = lg.With(zap.String("request_id", rid))
		}
		if ua := r.Header.Get("user-agent"); ua != "" {
			lg = lg.With(zap.String("user_agent", ua))
		}
		if r.RemoteAddr != "" {
			lg = lg.With(zap.String("client.addr", r.RemoteAddr))
		}

		lg.Info("http")
	})

	// IMPORTANT: wrap the accessLog handler with otelhttp so Context() has an active span.
	return otelhttp.NewHandler(accessLog, service)
}

type respWriter struct {
	http.ResponseWriter
	status int
}

func (w *respWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

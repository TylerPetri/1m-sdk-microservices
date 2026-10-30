package httpmw

import (
	"net/http"
)

// InFlightLimit applies backpressure by bounding the number of concurrent
// in-flight requests.
//
// When the limit is reached, it returns 503 immediately (fail-fast) rather than
// queueing unbounded work and risking OOM / tail-latency blowups.
func InFlightLimit(max int, next http.Handler) http.Handler {
	if max <= 0 {
		return next
	}

	sem := make(chan struct{}, max)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
			return
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
	})
}

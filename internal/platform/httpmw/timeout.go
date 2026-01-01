package httpmw

import (
	"context"
	"net/http"
	"time"
)

// Timeout enforces a per-request deadline.
//
// It only applies a deadline if the request context does not already have one.
// This means upstream callers (reverse proxies, gateways, etc.) can override it.
//
// If the deadline is exceeded, a 504 is returned.
func Timeout(d time.Duration, next http.Handler) http.Handler {
	if d <= 0 {
		return next
	}

	// Use net/http's TimeoutHandler so we always return a response even if the
	// downstream handler forgets to check ctx.Done().
	th := http.TimeoutHandler(next, d, http.StatusText(http.StatusGatewayTimeout))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); ok {
			// Respect upstream deadline.
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		th.ServeHTTP(w, r.WithContext(ctx))
	})
}

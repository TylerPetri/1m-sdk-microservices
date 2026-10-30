package httpmw

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// HeaderRequestID ensures every request has a request ID in the provided header.
// If absent, it generates one and always echoes it back on the response.
//
// This is a compatibility/helper wrapper so call sites can be explicit about the
// header they use (while still keeping one canonical implementation).
func EnsureRequestIDHeader(header string, next http.Handler) http.Handler {
	if header == "" {
		header = "X-Request-Id"
	}
	// Reuse our canonical RequestID middleware when the standard header is used.
	if header == "X-Request-Id" {
		return RequestID(next)
	}

	// Minimal custom-header variant.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(header) == "" {
			// Generate via our standard middleware by temporarily using X-Request-Id.
			// This keeps ID generation consistent (UUIDv4).
			tmp := r.Header.Get("X-Request-Id")
			r.Header.Set("X-Request-Id", "")
			RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rid := r.Header.Get("X-Request-Id")
				r.Header.Set(header, rid)
				if tmp != "" {
					r.Header.Set("X-Request-Id", tmp)
				}
				next.ServeHTTP(w, r)
			})).ServeHTTP(w, r)
			return
		}
		w.Header().Set(header, r.Header.Get(header))
		next.ServeHTTP(w, r)
	})
}

// RateLimit applies an in-memory per-client limiter.
func RateLimit(r rate.Limit, burst int, next http.Handler) http.Handler {
	const ttl = 10 * time.Minute
	return NewIPLimiter(r, burst, ttl).Wrap(next)
}

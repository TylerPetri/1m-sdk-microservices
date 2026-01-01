package httpmw

import (
	"net/http"

	"github.com/google/uuid"
)

// RequestID ensures every request has an X-Request-Id.
// If absent, it generates a UUIDv4. Always echoes back the header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-Id") == "" {
			r.Header.Set("X-Request-Id", uuid.NewString())
		}
		w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
		next.ServeHTTP(w, r)
	})
}

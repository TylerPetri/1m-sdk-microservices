package authctx

import (
	"net/http"
	"strings"
)

// GatewayAuth enforces Authorization header for all routes except the given public prefix.
// Example: publicPrefix="/v1/auth/" lets register/login through without auth.
func GatewayAuth(publicPrefix string, next http.Handler) http.Handler {
	if publicPrefix == "" {
		publicPrefix = "/"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// allow health endpoints through
		if path == "/healthz" || path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		// allow public auth routes through
		if strings.HasPrefix(path, publicPrefix) {
			next.ServeHTTP(w, r)
			return
		}

		// require Authorization for everything else
		if r.Header.Get("Authorization") == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

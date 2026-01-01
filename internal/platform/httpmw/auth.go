package httpmw

import (
	"net/http"
	"strings"

	"sdk-microservices/internal/platform/authctx"
	"sdk-microservices/internal/platform/authjwt"
)

// AuthBearer validates an Authorization: Bearer <token> header and stores user id in context.
// It does NOT enforce any specific audience; keep that in the JWT issuer/claims as needed.
func AuthBearer(jwtSvc *authjwt.Service, next http.Handler) http.Handler {
	if jwtSvc == nil {
		// If misconfigured, fail closed.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		tok := strings.TrimSpace(strings.TrimPrefix(h, prefix))
		if tok == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		claims, err := jwtSvc.Parse(tok)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		ctx := authctx.WithUserID(r.Context(), claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

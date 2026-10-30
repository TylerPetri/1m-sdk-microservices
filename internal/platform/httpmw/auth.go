package httpmw

import (
	"encoding/json"
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
			writeAuthError(w, http.StatusServiceUnavailable, 14, "authentication unavailable")
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			writeAuthError(w, http.StatusUnauthorized, 16, "unauthenticated")
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, http.StatusUnauthorized, 16, "unauthenticated")
			return
		}
		tok := parts[1]

		claims, err := jwtSvc.Parse(tok)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, 16, "unauthenticated")
			return
		}

		ctx := authctx.WithUserID(r.Context(), claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Match grpc-gateway's JSON status shape without leaking token/parser details.
func writeAuthError(w http.ResponseWriter, status, code int, message string) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{code, message})
}

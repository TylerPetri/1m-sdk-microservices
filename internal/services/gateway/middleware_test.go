package gateway

import (
	"net/http"
	"net/http/httptest"
	"sdk-microservices/internal/platform/authjwt"
	"testing"
	"time"
)

func TestEdgeAuthentication(t *testing.T) {
	var cfg Config
	cfg.JWT.Secret = "test-secret"
	cfg.JWT.Issuer = "test-issuer"
	cfg.Policy.Timeout = time.Second
	cfg.Policy.MaxInFlight = 10
	cfg.Policy.RateLimitRPS = 1000
	cfg.Policy.RateLimitBurst = 100
	signer := authjwt.New([]byte(cfg.JWT.Secret), cfg.JWT.Issuer, 0)
	valid, _, _ := signer.NewAccessToken("user", "test@example.com", time.Minute)
	expired, _, _ := signer.NewAccessToken("user", "test@example.com", -time.Minute)
	refresh, _, _ := signer.NewRefreshToken("user", "test@example.com", time.Minute)
	wrong, _, _ := authjwt.New([]byte("wrong"), cfg.JWT.Issuer, 0).NewAccessToken("user", "", time.Minute)
	h := BuildEdgeMiddleware(nil, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct {
		name, path, header string
		want               int
	}{
		{"missing", "/v1/hello/test", "", 401},
		{"arbitrary", "/v1/hello/test", "anything", 401},
		{"forged", "/v1/hello/test", "Bearer " + wrong, 401},
		{"expired", "/v1/hello/test", "Bearer " + expired, 401},
		{"refresh", "/v1/hello/test", "Bearer " + refresh, 401},
		{"valid", "/v1/hello/test", "Bearer " + valid, 204},
		{"login", "/v1/auth/login", "", 204},
		{"health", "/healthz", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

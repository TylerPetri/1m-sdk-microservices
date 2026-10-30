package gateway

import (
	"encoding/json"
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
	wrongIssuer, _, _ := authjwt.New([]byte(cfg.JWT.Secret), "other-issuer", 0).NewAccessToken("user", "", time.Minute)
	wrong, _, _ := authjwt.New([]byte("wrong"), cfg.JWT.Issuer, 0).NewAccessToken("user", "", time.Minute)
	h := BuildEdgeMiddleware(nil, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct {
		name, path, header string
		want               int
	}{
		{"malformed", "/v1/hello/test", "Bearer not-a-jwt", 401},
		{"empty-bearer", "/v1/hello/test", "Bearer ", 401},
		{"wrong-issuer", "/v1/hello/test", "Bearer " + wrongIssuer, 401},
		{"lowercase-scheme", "/v1/hello/test", "bearer " + valid, 204},
		{"extra-field", "/v1/hello/test", "Bearer " + valid + " ignored", 401},
		{"auth-prefix-lookalike", "/v1/authentic/hello", "", 401},
		{"auth-traversal", "/v1/auth/../hello/test", "", 401},
		{"encoded-auth-traversal", "/v1/auth/%2e%2e/hello/test", "", 401},
		{"encoded-slash", "/v1/auth%2f..%2fhello/test", "", 401},
		{"unknown-auth-route", "/v1/auth/private", "", 401},
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
			if tc.want == 401 {
				if w.Header().Get("WWW-Authenticate") == "" {
					t.Fatal("missing bearer challenge")
				}
				var body struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != 16 {
					t.Fatal("invalid JSON authentication status")
				}
			}
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

func TestDuplicateAuthorizationHeadersRejected(t *testing.T) {
	var cfg Config
	cfg.JWT.Secret = "test-secret"
	cfg.JWT.Issuer = "test-issuer"
	cfg.Policy.MaxInFlight = 10
	cfg.Policy.RateLimitRPS = 1000
	cfg.Policy.RateLimitBurst = 100
	token, _, err := authjwt.New([]byte(cfg.JWT.Secret), cfg.JWT.Issuer, 0).NewAccessToken("user", "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := BuildEdgeMiddleware(nil, cfg, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("duplicate headers reached handler") }))
	r := httptest.NewRequest("GET", "/v1/hello/test", nil)
	r.Header.Add("Authorization", "Bearer "+token)
	r.Header.Add("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status=%d", w.Code)
	}
}

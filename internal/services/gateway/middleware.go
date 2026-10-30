package gateway

import (
	"net/http"

	"sdk-microservices/internal/platform/authjwt"
	"sdk-microservices/internal/platform/httpmw"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// BuildEdgeMiddleware composes the canonical HTTP edge policy stack for gatewayd.
//
// Policy lives in Config; wiring lives here; cmd/gatewayd/main.go stays minimal.
func BuildEdgeMiddleware(log *zap.Logger, cfg Config, root http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}

	protected := httpmw.AuthBearer(authjwt.New([]byte(cfg.JWT.Secret), cfg.JWT.Issuer, 0), root)
	authenticated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/auth/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			root.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})

	// Order (outer -> inner):
	// - RequestID: ensure correlation for all other middleware + downstream
	// - Recover: panic safety
	// - SecurityHeaders: sane defaults
	// - Timeout: upper bound per request
	// - InFlightLimit: protect process
	// - RateLimit: protect edge
	// - Auth: auth-gate selected prefixes
	return httpmw.RequestID(
		httpmw.Recover(log,
			httpmw.SecurityHeaders(
				httpmw.Timeout(cfg.Policy.Timeout,
					httpmw.InFlightLimit(cfg.Policy.MaxInFlight,
						httpmw.RateLimit(
							rate.Limit(cfg.Policy.RateLimitRPS),
							cfg.Policy.RateLimitBurst,
							authenticated,
						),
					),
				),
			),
		),
	)
}

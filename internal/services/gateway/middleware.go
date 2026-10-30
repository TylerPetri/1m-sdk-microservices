package gateway

import (
	"net/http"

	"sdk-microservices/internal/platform/authctx"
	"sdk-microservices/internal/platform/httpmw"

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
							authctx.GatewayAuth("/v1/auth/", root),
						),
					),
				),
			),
		),
	)
}

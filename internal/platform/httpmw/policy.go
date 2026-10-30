package httpmw

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// EdgePolicy defines a "serious default" HTTP middleware policy for public-facing services.
// It's intended to be configuration-driven, so adding a new service doesn't require
// copy/pasting a nested middleware stack.
type EdgePolicy struct {
	// ServiceName is used for OpenTelemetry span names + access log fields.
	ServiceName string

	// Timeout bounds total handler time.
	Timeout time.Duration

	// MaxInFlight limits concurrent requests processed by the server handler.
	MaxInFlight int

	// Outer is applied outside the default edge chain (i.e., even before RequestID/Recover).
	// Use sparingly.
	Outer Chain

	// Leaf is applied closest to the business handler, inside the default edge chain.
	// Typical examples: rate limiting, auth, request validation, etc.
	Leaf Chain
}

// DefaultEdge returns the default "edge" chain, excluding Wrap() and excluding any leaf middleware.
func DefaultEdge(log *zap.Logger, timeout time.Duration, maxInFlight int) Chain {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxInFlight <= 0 {
		maxInFlight = 512
	}

	return Chain{
		RequestID,
		WithRecover(log),
		SecurityHeaders,
		WithTimeout(timeout),
		WithInFlightLimit(maxInFlight),
	}
}

// BuildEdgeHandler composes a policy-driven middleware stack around next.
//
// Final order (outer -> inner):
//
//	Outer..., Wrap, RequestID, Recover, SecurityHeaders, Timeout, InFlightLimit, Leaf..., next
func BuildEdgeHandler(log *zap.Logger, p EdgePolicy, next http.Handler) http.Handler {
	if p.ServiceName == "" {
		p.ServiceName = "service"
	}

	leaf := p.Leaf.Then(next)

	core := DefaultEdge(log, p.Timeout, p.MaxInFlight).
		Append() // no-op; keeps style consistent

	h := core.Then(leaf)

	// Add standard tracing + access logging outside of the default policy chain.
	h = WithWrap(p.ServiceName, log)(h)

	// Finally apply any outer middleware.
	h = p.Outer.Then(h)

	return h
}

package httpmw

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Middleware is a standard net/http middleware signature.
type Middleware func(http.Handler) http.Handler

// Chain is an ordered list of middleware.
// The first element is treated as the outermost wrapper.
type Chain []Middleware

// Then applies the middleware chain to h and returns the wrapped handler.
func (c Chain) Then(h http.Handler) http.Handler {
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == nil {
			continue
		}
		h = c[i](h)
	}
	return h
}

// Append returns a new chain with additional middleware appended (as new innermost entries).
func (c Chain) Append(mw ...Middleware) Chain {
	out := make(Chain, 0, len(c)+len(mw))
	out = append(out, c...)
	out = append(out, mw...)
	return out
}

// WithWrap adapts Wrap(service, log, next) into a Middleware.
func WithWrap(service string, log *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return Wrap(service, log, next)
	}
}

// WithRecover adapts Recover(log, next) into a Middleware.
func WithRecover(log *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return Recover(log, next)
	}
}

// WithTimeout adapts Timeout(d, next) into a Middleware.
func WithTimeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return Timeout(d, next)
	}
}

// WithInFlightLimit adapts InFlightLimit(max, next) into a Middleware.
func WithInFlightLimit(max int) Middleware {
	return func(next http.Handler) http.Handler {
		return InFlightLimit(max, next)
	}
}

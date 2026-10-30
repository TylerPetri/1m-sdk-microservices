package authctx

import "context"

type ctxKey struct{}

// WithUserID stores an authenticated user id in context.
func WithUserID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserID returns the authenticated user id, if present.
func UserID(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxKey{})
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}

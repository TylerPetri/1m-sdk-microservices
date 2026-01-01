package jwt

import "sdk-microservices/internal/platform/authjwt"

// Deprecated: moved to internal/platform/authjwt. This file exists to keep refactors mechanical.
type Service = authjwt.Service
type Claims = authjwt.Claims

var (
	ErrInvalidToken = authjwt.ErrInvalidToken
)

func New(secret, issuer string, ttlSeconds int64) *Service {
	return authjwt.New([]byte(secret), issuer, ttlSeconds)
}

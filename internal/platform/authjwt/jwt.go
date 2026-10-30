package authjwt

import (
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
)

type Service struct {
	secret []byte
	issuer string
	ttl    int64
}

func New(secret []byte, issuer string, ttl int64) *Service {
	return &Service{secret: secret, issuer: issuer, ttl: ttl}
}

type Claims struct {
	Email string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

func (s *Service) NewAccessToken(userID, email string, ttl time.Duration) (token string, exp time.Time, err error) {
	now := time.Now().UTC()
	exp = now.Add(ttl)

	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign: %w", err)
	}
	return signed, exp, nil
}

func (s *Service) NewRefreshToken(userID, email string, ttl time.Duration) (string, time.Time, error) {
	// For now, refresh token is also a JWT with a longer TTL.
	// Later we can add rotation + DB-backed revocation.
	now := time.Now().UTC()
	exp := now.Add(ttl)

	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        "refresh",
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign: %w", err)
	}
	return signed, exp, nil
}

func (s *Service) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Issuer != s.issuer {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

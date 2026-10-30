package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// NewRefreshToken returns a new opaque refresh token.
//
// 32 random bytes -> base64url(no padding) gives a compact string safe for cookies/JSON.
func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashRefreshToken returns the sha256 hash of the opaque refresh token.
// Store only this value in the database.
func HashRefreshToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

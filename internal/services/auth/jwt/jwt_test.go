package jwt

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	s := New("secret", "issuer")
	tok, exp, err := s.NewAccessToken("user-123", "u@example.com", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewAccessToken err=%v", err)
	}
	if tok == "" {
		t.Fatalf("expected non-empty token")
	}
	if exp.Before(time.Now().Add(30 * time.Second)) {
		t.Fatalf("exp too soon: %v", exp)
	}

	claims, err := s.Parse(tok)
	if err != nil {
		t.Fatalf("Parse err=%v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("subject=%q", claims.Subject)
	}
	if claims.Email != "u@example.com" {
		t.Fatalf("email=%q", claims.Email)
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	a := New("secret", "issuer-a")
	b := New("secret", "issuer-b")
	tok, _, err := a.NewAccessToken("user-123", "u@example.com", time.Minute)
	if err != nil {
		t.Fatalf("NewAccessToken err=%v", err)
	}
	if _, err := b.Parse(tok); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	s := New("secret", "issuer")
	if _, err := s.Parse("not-a-jwt"); err == nil {
		t.Fatalf("expected parse error")
	}
}

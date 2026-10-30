package authjwt_test

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"sdk-microservices/internal/platform/authjwt"
	authservice "sdk-microservices/internal/services/auth/jwt"
)

func TestAccessTokenValidationContract(t *testing.T) {
	const key = "contract-test-key"
	const issuer = "contract-issuer"
	sign := func(method jwtlib.SigningMethod, claims jwtlib.MapClaims, signingKey any) string {
		t.Helper()
		token, err := jwtlib.NewWithClaims(method, claims).SignedString(signingKey)
		if err != nil {
			t.Fatal("sign test token:", err)
		}
		return token
	}
	claims := func() jwtlib.MapClaims {
		return jwtlib.MapClaims{"iss": issuer, "sub": "user", "exp": time.Now().Add(time.Minute).Unix()}
	}
	valid := sign(jwtlib.SigningMethodHS256, claims(), []byte(key))
	type tokenCase struct {
		name, token string
		valid       bool
	}
	cases := []tokenCase{{"valid", valid, true}, {"malformed", "not-a-jwt", false}, {"broken-base64", "@@@.@@@.@@@", false}, {"empty", "", false}}
	for _, change := range []struct {
		name, field string
		value       any
	}{
		{"expired", "exp", time.Now().Add(-time.Minute).Unix()},
		{"missing-expiry", "exp", nil}, {"invalid-expiry", "exp", "tomorrow"},
		{"wrong-issuer", "iss", "other"}, {"missing-issuer", "iss", nil},
		{"missing-subject", "sub", nil}, {"blank-subject", "sub", " "},
		{"future-not-before", "nbf", time.Now().Add(time.Hour).Unix()},
		{"refresh-token", "jti", "refresh"},
	} {
		c := claims()
		if change.value == nil {
			delete(c, change.field)
		} else {
			c[change.field] = change.value
		}
		cases = append(cases, tokenCase{change.name, sign(jwtlib.SigningMethodHS256, c, []byte(key)), false})
	}
	cases = append(cases,
		tokenCase{"invalid-signature", sign(jwtlib.SigningMethodHS256, claims(), []byte("wrong-key")), false},
		tokenCase{"wrong-algorithm", sign(jwtlib.SigningMethodHS384, claims(), []byte(key)), false},
		tokenCase{"unsigned", sign(jwtlib.SigningMethodNone, claims(), jwtlib.UnsafeAllowNoneSignatureType), false})
	edge := authjwt.New([]byte(key), issuer, 0)
	service := authservice.New(key, issuer)
	for _, parser := range []struct {
		name  string
		parse func(string) error
	}{
		{"gateway", func(token string) error { _, err := edge.Parse(token); return err }},
		{"auth-service", func(token string) error { _, err := service.Parse(token); return err }},
	} {
		for _, tc := range cases {
			t.Run(parser.name+"/"+tc.name, func(t *testing.T) {
				err := parser.parse(tc.token)
				if (err == nil) != tc.valid {
					t.Fatalf("accepted=%v want=%v", err == nil, tc.valid)
				}
			})
		}
	}
	// The issuing service and gateway must agree on the actual access/refresh formats.
	access, _, err := service.NewAccessToken("user", "u@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = edge.Parse(access); err != nil {
		t.Fatal("gateway rejected issued access token")
	}
	refresh, _, err := service.NewRefreshToken("user", "u@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = edge.Parse(refresh); err == nil {
		t.Fatal("gateway accepted issued refresh token")
	}
}

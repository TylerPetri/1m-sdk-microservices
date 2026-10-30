package gateway_test

import (
	"sdk-microservices/internal/platform/config"
	authcfg "sdk-microservices/internal/services/auth"
	gatewaycfg "sdk-microservices/internal/services/gateway"
	"strings"
	"testing"
)

func TestSigningConfigurationFailsClosed(t *testing.T) {
	for _, env := range []string{"dev", "staging", "prod"} {
		for _, key := range []string{"", "short", strings.Repeat("x", 32)} {
			t.Run(env+"/key-length-"+string(rune('A'+len(key))), func(t *testing.T) {
				t.Setenv("APP_ENV", env)
				t.Setenv("AUTH_JWT_SECRET", key)
				t.Setenv("AUTH_DB_DSN", "postgres://unused-test-only/auth")
				_, authErr := config.Load[authcfg.Config]()
				_, gatewayErr := config.Load[gatewaycfg.Config]()
				wantValid := len(key) >= 32
				if (authErr == nil) != wantValid || (gatewayErr == nil) != wantValid {
					t.Fatalf("auth accepted=%v gateway accepted=%v want=%v", authErr == nil, gatewayErr == nil, wantValid)
				}
			})
		}
	}
}

func TestProdDatabaseMustBeExplicit(t *testing.T) {
	for _, env := range []string{"staging", "prod", "PROD"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("AUTH_JWT_SECRET", strings.Repeat("x", 32))
			t.Setenv("AUTH_DB_DSN", "")
			if _, err := config.Load[authcfg.Config](); err == nil {
				t.Fatal("accepted implicit development database")
			}
		})
	}
}

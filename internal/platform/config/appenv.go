package config

import (
	"fmt"
	"strings"
)

// AuthenticateAppEnv normalizes and validates the APP_ENV value.
//
// Rules:
// - Empty => "dev"
// - Case-insensitive
// - Allowed: dev, staging, prod
func AuthenticateAppEnv(raw string) (AppEnv, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return EnvDev, nil
	}
	switch s {
	case string(EnvDev):
		return EnvDev, nil
	case string(EnvStaging):
		return EnvStaging, nil
	case string(EnvProd):
		return EnvProd, nil
	default:
		return "", fmt.Errorf("APP_ENV must be one of dev|staging|prod (got %q)", raw)
	}
}

func (e AppEnv) IsProdLike() bool {
	return e == EnvStaging || e == EnvProd
}

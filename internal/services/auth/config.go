package auth

import (
	"fmt"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/config"
	"strings"
	"time"
)

type Config struct {
	Platform boot.PlatformConfig `envPrefix:""`

	GRPC struct {
		Addr           string        `env:"AUTH_ADDR" default:":50052"`
		DefaultTimeout time.Duration `env:"AUTH_RPC_TIMEOUT" default:"10s"`
		MaxInFlight    int           `env:"AUTH_MAX_INFLIGHT" default:"256"`
	} `envPrefix:""`

	Admin struct {
		Addr string `env:"AUTH_ADMIN_ADDR" default:":8081"`
	} `envPrefix:""`

	DB struct {
		DSN  string `env:"AUTH_DB_DSN" default:"postgres://postgres:postgres@localhost:5432/auth?sslmode=disable" required_in:"staging,prod" redact:"true"`
		Pool struct {
			MaxConns          int           `env:"AUTH_DB_MAX_CONNS" default:"20"`
			MinConns          int           `env:"AUTH_DB_MIN_CONNS" default:"2"`
			MaxConnLifetime   time.Duration `env:"AUTH_DB_MAX_CONN_LIFETIME" default:"30m"`
			MaxConnIdleTime   time.Duration `env:"AUTH_DB_MAX_CONN_IDLE" default:"5m"`
			HealthCheckPeriod time.Duration `env:"AUTH_DB_HEALTHCHECK" default:"30s"`
		} `envPrefix:""`
	} `envPrefix:""`

	JWT struct {
		Secret string `env:"AUTH_JWT_SECRET" default:"dev-secret-change-me" required_in:"staging,prod" redact:"true"`
		Issuer string `env:"AUTH_JWT_ISSUER" default:"sdk-microservices"`
	} `envPrefix:""`

	Tokens struct {
		AccessTTL  time.Duration `env:"AUTH_ACCESS_TTL" default:"15m"`
		RefreshTTL time.Duration `env:"AUTH_REFRESH_TTL" default:"168h"` // 7d
	} `envPrefix:""`
}

type GRPCConfig struct {
	Addr           string        `env:"AUTH_ADDR" default:":50052"`
	DefaultTimeout time.Duration `env:"AUTH_RPC_TIMEOUT" default:"10s"`
	MaxInFlight    int           `env:"AUTH_MAX_INFLIGHT" default:"256"`
}

type AdminConfig struct {
	Addr string `env:"AUTH_ADMIN_ADDR" default:":8081"`
}

type DBConfig struct {
	DSN  string       `env:"AUTH_DB_DSN" default:"postgres://postgres:postgres@localhost:5432/auth?sslmode=disable" required_in:"staging,prod" redact:"true"`
	Pool DBPoolConfig `envPrefix:""`
}

type DBPoolConfig struct {
	MaxConns          int           `env:"AUTH_DB_MAX_CONNS" default:"20"`
	MinConns          int           `env:"AUTH_DB_MIN_CONNS" default:"2"`
	MaxConnLifetime   time.Duration `env:"AUTH_DB_MAX_CONN_LIFETIME" default:"30m"`
	MaxConnIdleTime   time.Duration `env:"AUTH_DB_MAX_CONN_IDLE" default:"5m"`
	HealthCheckPeriod time.Duration `env:"AUTH_DB_HEALTHCHECK" default:"30s"`
}

type JWTConfig struct {
	Secret string `env:"AUTH_JWT_SECRET" default:"dev-secret-change-me" required_in:"staging,prod" redact:"true"`
	Issuer string `env:"AUTH_JWT_ISSUER" default:"sdk-microservices"`
}

type TokenConfig struct {
	AccessTTL  time.Duration `env:"AUTH_ACCESS_TTL" default:"15m"`
	RefreshTTL time.Duration `env:"AUTH_REFRESH_TTL" default:"168h"`
}

func (c Config) Validate() error {
	appEnv, err := config.AuthenticateAppEnv(c.Platform.Env.AppEnv)
	if err != nil {
		return err
	}

	// GRPC/Admin
	if strings.TrimSpace(c.GRPC.Addr) == "" {
		return fmt.Errorf("AUTH_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Admin.Addr) == "" {
		return fmt.Errorf("AUTH_ADMIN_ADDR must not be empty")
	}
	if c.GRPC.DefaultTimeout <= 0 {
		return fmt.Errorf("AUTH_RPC_TIMEOUT must be > 0")
	}
	if c.GRPC.MaxInFlight < 1 {
		return fmt.Errorf("AUTH_MAX_INFLIGHT must be >= 1")
	}

	// DB
	if strings.TrimSpace(c.DB.DSN) == "" {
		return fmt.Errorf("AUTH_DB_DSN must not be empty")
	}
	if c.DB.Pool.MaxConns < 1 {
		return fmt.Errorf("AUTH_DB_MAX_CONNS must be >= 1")
	}
	if c.DB.Pool.MinConns < 0 {
		return fmt.Errorf("AUTH_DB_MIN_CONNS must be >= 0")
	}
	if c.DB.Pool.MinConns > c.DB.Pool.MaxConns {
		return fmt.Errorf("AUTH_DB_MIN_CONNS must be <= AUTH_DB_MAX_CONNS")
	}
	if c.DB.Pool.MaxConnLifetime <= 0 {
		return fmt.Errorf("AUTH_DB_MAX_CONN_LIFETIME must be > 0")
	}
	if c.DB.Pool.MaxConnIdleTime <= 0 {
		return fmt.Errorf("AUTH_DB_MAX_CONN_IDLE must be > 0")
	}
	if c.DB.Pool.HealthCheckPeriod <= 0 {
		return fmt.Errorf("AUTH_DB_HEALTHCHECK must be > 0")
	}

	// JWT
	if strings.TrimSpace(c.JWT.Issuer) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER must not be empty")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("AUTH_JWT_SECRET must not be empty")
	}
	if string(appEnv) == "staging" || string(appEnv) == "prod" {
		if len(c.JWT.Secret) < 32 {
			return fmt.Errorf("AUTH_JWT_SECRET must be at least 32 characters in staging/prod")
		}
		if c.JWT.Secret == "dev-secret-change-me" {
			return fmt.Errorf("AUTH_JWT_SECRET must be set to a non-default value in staging/prod")
		}
	}

	// Tokens
	if c.Tokens.AccessTTL <= 0 {
		return fmt.Errorf("AUTH_ACCESS_TTL must be > 0")
	}
	if c.Tokens.RefreshTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_TTL must be > 0")
	}
	if c.Tokens.RefreshTTL <= c.Tokens.AccessTTL {
		return fmt.Errorf("AUTH_REFRESH_TTL must be > AUTH_ACCESS_TTL")
	}

	return nil
}

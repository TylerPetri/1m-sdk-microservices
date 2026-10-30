package gateway

import (
	"fmt"
	"sdk-microservices/internal/platform/boot"
	"sdk-microservices/internal/platform/config"
	"strings"
	"time"
)

type Config struct {
	JWT struct {
		Secret string `env:"AUTH_JWT_SECRET" default:"dev-secret-change-me" required_in:"staging,prod" redact:"true"`
		Issuer string `env:"AUTH_JWT_ISSUER" default:"sdk-microservices"`
	}

	Platform boot.PlatformConfig `envPrefix:""`

	HTTP struct {
		Addr string `env:"GATEWAY_HTTP_ADDR" default:":8080"`
	} `envPrefix:""`

	Admin struct {
		Addr string `env:"GATEWAY_ADMIN_ADDR" default:":8081"`
	} `envPrefix:""`

	Upstreams struct {
		HelloGRPCAddr string `env:"HELLO_GRPC_ADDR" default:"localhost:50051"`
		AuthGRPCAddr  string `env:"AUTH_GRPC_ADDR" default:"localhost:50052"`
	} `envPrefix:""`

	Policy struct {
		Timeout        time.Duration `env:"GATEWAY_TIMEOUT" default:"30s"`
		MaxInFlight    int           `env:"GATEWAY_MAX_INFLIGHT" default:"512"`
		RateLimitRPS   float64       `env:"GATEWAY_RATELIMIT_RPS" default:"200"`
		RateLimitBurst int           `env:"GATEWAY_RATELIMIT_BURST" default:"400"`
	} `envPrefix:""`
}

type HTTPConfig struct {
	Addr string `env:"GATEWAY_HTTP_ADDR" default:":8080"`
}

type AdminConfig struct {
	Addr string `env:"GATEWAY_ADMIN_ADDR" default:":8081"`
}

type UpstreamsConfig struct {
	HelloGRPCAddr string `env:"HELLO_GRPC_ADDR" default:"localhost:50051"`
	AuthGRPCAddr  string `env:"AUTH_GRPC_ADDR" default:"localhost:50052"`
}

type PolicyConfig struct {
	Timeout        time.Duration `env:"GATEWAY_TIMEOUT" default:"30s"`
	MaxInFlight    int           `env:"GATEWAY_MAX_INFLIGHT" default:"512"`
	RateLimitRPS   float64       `env:"GATEWAY_RATELIMIT_RPS" default:"200"`
	RateLimitBurst int           `env:"GATEWAY_RATELIMIT_BURST" default:"400"`
}

func (c Config) Validate() error {
	appEnv, err := config.AuthenticateAppEnv(c.Platform.Env.AppEnv)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.JWT.Secret) == "" || strings.TrimSpace(c.JWT.Issuer) == "" {
		return fmt.Errorf("JWT secret and issuer must not be empty")
	}
	if (string(appEnv) == "prod" || string(appEnv) == "staging") && (len(c.JWT.Secret) < 32 || c.JWT.Secret == "dev-secret-change-me") {
		return fmt.Errorf("AUTH_JWT_SECRET must be a non-default value of at least 32 characters in staging/prod")
	}
	if strings.TrimSpace(c.HTTP.Addr) == "" {
		return fmt.Errorf("GATEWAY_HTTP_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Admin.Addr) == "" {
		return fmt.Errorf("GATEWAY_ADMIN_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Upstreams.HelloGRPCAddr) == "" {
		return fmt.Errorf("HELLO_GRPC_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Upstreams.AuthGRPCAddr) == "" {
		return fmt.Errorf("AUTH_GRPC_ADDR must not be empty")
	}
	if c.Policy.Timeout <= 0 {
		return fmt.Errorf("GATEWAY_TIMEOUT must be > 0")
	}
	if c.Policy.MaxInFlight < 1 {
		return fmt.Errorf("GATEWAY_MAX_INFLIGHT must be >= 1")
	}
	if c.Policy.RateLimitRPS <= 0 {
		return fmt.Errorf("GATEWAY_RATELIMIT_RPS must be > 0")
	}
	if c.Policy.RateLimitBurst < 1 {
		return fmt.Errorf("GATEWAY_RATELIMIT_BURST must be >= 1")
	}
	return nil
}

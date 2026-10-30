package hello

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
		Addr           string        `env:"HELLO_ADDR" default:":50051"`
		DefaultTimeout time.Duration `env:"HELLO_RPC_TIMEOUT" default:"10s"`
		MaxInFlight    int           `env:"HELLO_MAX_INFLIGHT" default:"256"`
	} `envPrefix:""`

	Admin struct {
		Addr string `env:"HELLO_ADMIN_ADDR" default:":8081"`
	} `envPrefix:""`
}

type GRPCConfig struct {
	Addr           string        `env:"HELLO_ADDR" default:":50051"`
	DefaultTimeout time.Duration `env:"HELLO_RPC_TIMEOUT" default:"10s"`
	MaxInFlight    int           `env:"HELLO_MAX_INFLIGHT" default:"256"`
}

type AdminConfig struct {
	Addr string `env:"HELLO_ADMIN_ADDR" default:":8081"`
}

func (c Config) Validate() error {
	_, err := config.AuthenticateAppEnv(c.Platform.Env.AppEnv)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.GRPC.Addr) == "" {
		return fmt.Errorf("HELLO_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Admin.Addr) == "" {
		return fmt.Errorf("HELLO_ADMIN_ADDR must not be empty")
	}
	if c.GRPC.DefaultTimeout <= 0 {
		return fmt.Errorf("HELLO_RPC_TIMEOUT must be > 0")
	}
	if c.GRPC.MaxInFlight < 1 {
		return fmt.Errorf("HELLO_MAX_INFLIGHT must be >= 1")
	}
	return nil
}

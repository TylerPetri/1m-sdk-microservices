package boot

import "sdk-microservices/internal/platform/otel"

type PlatformConfig struct {
	Env  EnvConfig   `envPrefix:""`
	OTel otel.Config `envPrefix:""`
}

type EnvConfig struct {
	AppEnv       string `env:"APP_ENV" default:"dev"`
	LogEffective bool   `env:"CONFIG_LOG_EFFECTIVE" default:"false"`
}

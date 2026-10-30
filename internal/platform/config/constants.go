package config

// AppEnv is the normalized application environment value.
type AppEnv string

const (
	EnvDev     AppEnv = "dev"
	EnvStaging AppEnv = "staging"
	EnvProd    AppEnv = "prod"
)

package logging

import "go.uber.org/zap"

func New(service string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.InitialFields = map[string]any{"service": service}
	return cfg.Build()
}

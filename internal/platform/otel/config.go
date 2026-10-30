package otel

import "fmt"

type Config struct {
	// If empty, exporter falls back to stdout (dev-friendly).
	Endpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" default:""`

	// Allowed: "", "grpc", "http", "http/protobuf"
	Protocol string `env:"OTEL_EXPORTER_OTLP_PROTOCOL" default:""`

	// For grpc only. Default false.
	Insecure bool `env:"OTEL_EXPORTER_OTLP_INSECURE" default:"false"`
}

func (c Config) Validate() error {
	if c.Endpoint == "" {
		return nil
	}
	switch c.Protocol {
	case "", "grpc", "http", "http/protobuf":
		// ok
	default:
		return fmt.Errorf("OTEL_EXPORTER_OTLP_PROTOCOL must be one of grpc|http|http/protobuf (got %q)", c.Protocol)
	}
	if c.Insecure && c.Protocol != "" && c.Protocol != "grpc" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_INSECURE only applies to grpc protocol")
	}
	return nil
}

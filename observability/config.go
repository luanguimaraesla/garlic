package observability

import "time"

// Config configures the global metrics provider installed by [Init].
//
// All exporter fields are optional. When left at their zero value the standard
// OpenTelemetry environment variables apply (OTEL_EXPORTER_OTLP_ENDPOINT,
// OTEL_EXPORTER_OTLP_INSECURE, OTEL_METRIC_EXPORT_INTERVAL, ...). A non-zero
// field overrides the corresponding environment variable.
type Config struct {
	// ServiceName is reported as the service.name resource attribute. Defaults
	// to "garlic".
	ServiceName string `json:"service_name" mapstructure:"service_name" yaml:"service_name"`

	// Endpoint is the OTLP/gRPC collector address ("host:port"). Overrides
	// OTEL_EXPORTER_OTLP_ENDPOINT when set.
	Endpoint string `json:"endpoint" mapstructure:"endpoint" yaml:"endpoint"`

	// Insecure disables transport security (plaintext gRPC). When true it
	// overrides OTEL_EXPORTER_OTLP_INSECURE; when false the environment variable
	// still applies.
	Insecure bool `json:"insecure" mapstructure:"insecure" yaml:"insecure"`

	// Interval is the period between metric exports. Overrides
	// OTEL_METRIC_EXPORT_INTERVAL when set (the SDK default is 60s).
	Interval time.Duration `json:"interval" mapstructure:"interval" yaml:"interval"`
}

// Defaults returns a Config with sensible defaults. Exporter fields are left
// empty so the standard OpenTelemetry environment variables apply.
func Defaults() *Config {
	return &Config{
		ServiceName: "garlic",
	}
}

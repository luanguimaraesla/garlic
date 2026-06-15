package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/global"
	"github.com/luanguimaraesla/garlic/logging"
)

var singleton *sdkmetric.MeterProvider

// Init installs a global OpenTelemetry MeterProvider that pushes metrics to an
// OTLP/gRPC collector, so the metrics recorded by middleware.MetricsMonitor are
// exported on an interval.
//
// Call Init once, early at startup, before serving traffic. Calling it more
// than once is fatal. If config is nil, [Defaults] are used. Exporter settings
// come from the standard OTEL_EXPORTER_OTLP_* environment variables unless
// overridden by non-zero [Config] fields.
func Init(config *Config) {
	if singleton != nil {
		logging.Global().Fatal("Failed to initialize observability: this is already set")
	}

	if config == nil {
		config = Defaults()
	}

	exporter, err := otlpmetricgrpc.New(context.Background(), exporterOptions(config)...)
	if err != nil {
		logging.Global().Fatal("Failed to create OTLP metrics exporter", errors.Zap(err))
	}

	singleton = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, readerOptions(config)...)),
		sdkmetric.WithResource(buildResource(config)),
	)

	otel.SetMeterProvider(singleton)
}

// buildResource describes this service. It merges with resource.Default() so the
// telemetry.sdk.* attributes and the OTEL_RESOURCE_ATTRIBUTES / OTEL_SERVICE_NAME
// environment variables are honored. Configured values take precedence; service
// name is left to the environment when ServiceName is empty.
func buildResource(config *Config) *resource.Resource {
	attrs := []attribute.KeyValue{semconv.ServiceVersion(global.Version)}
	if config.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(config.ServiceName))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
	if err != nil {
		logging.Global().Warn(
			"Telemetry resource has conflicting schema URLs",
			errors.Zap(errors.Propagate(err, "failed to merge telemetry resource")),
		)
	}

	return res
}

// Shutdown flushes pending metrics and releases the MeterProvider. Call it
// during graceful shutdown so the last interval of metrics is exported; it
// pairs well with rest.WithOnShutdown. It is a no-op if [Init] has not been
// called, and it clears the singleton so [Init] may be called again afterwards.
func Shutdown(ctx context.Context) error {
	if singleton == nil {
		return nil
	}

	mp := singleton
	singleton = nil

	if err := mp.ForceFlush(ctx); err != nil {
		return errors.Propagate(err, "failed to flush metrics")
	}

	if err := mp.Shutdown(ctx); err != nil {
		return errors.Propagate(err, "failed to shut down meter provider")
	}

	return nil
}

// exporterOptions translates the Config into OTLP exporter options. Empty
// fields are omitted so the standard OTEL environment variables apply.
func exporterOptions(config *Config) []otlpmetricgrpc.Option {
	var opts []otlpmetricgrpc.Option

	if config.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(config.Endpoint))
	}

	if config.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	return opts
}

// readerOptions translates the Config into periodic reader options. A zero
// interval falls back to OTEL_METRIC_EXPORT_INTERVAL or the SDK default.
func readerOptions(config *Config) []sdkmetric.PeriodicReaderOption {
	var opts []sdkmetric.PeriodicReaderOption

	if config.Interval > 0 {
		opts = append(opts, sdkmetric.WithInterval(config.Interval))
	}

	return opts
}

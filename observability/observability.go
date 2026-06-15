package observability

import (
	"context"

	"go.opentelemetry.io/otel"
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

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(global.Version),
	)

	singleton = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, readerOptions(config)...)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(singleton)
}

// Shutdown flushes pending metrics and releases the MeterProvider. Call it
// during graceful shutdown so the last interval of metrics is exported; it
// pairs well with rest.WithOnShutdown. It is a no-op if [Init] has not been
// called.
func Shutdown(ctx context.Context) error {
	if singleton == nil {
		return nil
	}

	if err := singleton.ForceFlush(ctx); err != nil {
		return errors.Propagate(err, "failed to flush metrics")
	}

	if err := singleton.Shutdown(ctx); err != nil {
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

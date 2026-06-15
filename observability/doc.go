// Package observability installs an OpenTelemetry MeterProvider that pushes
// metrics to an OTLP/gRPC collector.
//
// garlic's HTTP metrics are recorded through the global OpenTelemetry meter
// (see the monitoring package). By default that provider is a no-op, so nothing
// is exported until an application installs a real one. This package provides
// the recommended one-liner for sending metrics to an OpenTelemetry collector.
//
// # Initialization
//
// Call [Init] once at startup, before serving traffic:
//
//	observability.Init(&observability.Config{ServiceName: "myservice"})
//
// [Init] builds a MeterProvider with an OTLP/gRPC exporter behind a periodic
// reader and installs it via otel.SetMeterProvider. Calling [Init] more than
// once panics. The service version is taken from
// [github.com/luanguimaraesla/garlic/global.Version].
//
// # Configuration
//
// Exporter settings come from the standard OpenTelemetry environment variables
// (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_INSECURE,
// OTEL_METRIC_EXPORT_INTERVAL, ...). Non-zero [Config] fields override the
// corresponding variable, so a typical collector-sidecar deployment needs no
// code configuration beyond the service name:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
//
// # Shutdown
//
// Call [Shutdown] during graceful shutdown to flush the last interval of
// metrics and release the provider; it pairs well with rest.WithOnShutdown:
//
//	rest.GetServer("api", rest.WithOnShutdown(func(ctx context.Context) {
//	    _ = observability.Shutdown(ctx)
//	}))
package observability

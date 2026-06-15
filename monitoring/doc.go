// Package monitoring records HTTP request metrics through OpenTelemetry.
//
// Three instruments are recorded under the meter
// "github.com/luanguimaraesla/garlic" via the global MeterProvider:
//
//   - http.server.requests — Int64Counter (attributes: http.request.method,
//     http.route, http.response.status_code)
//   - http.server.active_requests — Int64UpDownCounter (attributes:
//     http.request.method, http.route)
//   - http.server.request.duration — Float64Histogram in seconds (attributes:
//     http.request.method, http.route, http.response.status_code)
//
// Helper functions [IncrementTraffic], [IncrementActiveRequests],
// [DecrementActiveRequests], and [ObserveLatency] record against these
// instruments and are called automatically by the middleware.MetricsMonitor
// middleware.
//
// The instruments are created lazily on first use from the global
// MeterProvider, so they bind to whichever provider the application installs at
// startup. Until a provider is installed the default is a no-op and nothing is
// exported. The observability package provides a one-line setup that installs
// a MeterProvider which pushes the metrics to an OTLP collector.
package monitoring

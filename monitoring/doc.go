// Package monitoring registers Prometheus metrics for HTTP request tracking.
//
// Three metric vectors are registered as a side effect of importing this
// package:
//
//   - [TrafficMetric] — counter http_request_total (labels: method, route, status_code)
//   - [ActiveRequests] — gauge http_active_requests (labels: method, route)
//   - [LatencyMetric] — histogram http_request_duration_seconds (labels: method, route)
//
// Helper functions [IncrementTraffic], [IncrementActiveRequests],
// [DecrementActiveRequests], and [ObserveLatency] provide label-safe access.
// These are called automatically by the middleware.MetricsMonitor middleware.
package monitoring

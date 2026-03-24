// Package middleware provides HTTP middleware for logging, tracing, monitoring,
// CORS, content-type enforcement, and context cancellation.
//
// Each middleware is a standard [net/http.Handler] wrapper compatible with
// Chi's Use method.
//
// # Available Middleware
//
//   - [ContextCancel] — creates a cancellable child context, ensuring cleanup
//     after each request.
//   - [Logging] — injects a structured logger into the request context and logs
//     method, URL, status code, response size, and duration.
//   - [Tracing] — generates a UUID request ID, sets the X-Request-ID response
//     header, and stores it in context. [PropagateTracing] reads existing
//     X-Request-ID and X-Session-ID headers from upstream services.
//   - [MetricsMonitor] — records Prometheus metrics: http_request_total,
//     http_active_requests, and http_request_duration_seconds.
//   - [ContentTypeJson] — sets Content-Type: application/json on every response.
//   - [Cors] — sets CORS headers from a [Config] and handles OPTIONS preflight.
//
// # Recommended Stack Order
//
// Apply middleware in this order so that each layer has access to the context
// values set by previous layers:
//
//	router.Use(
//	    middleware.ContextCancel,    // cancellable context for all handlers
//	    middleware.Logging,         // logger injected into context
//	    middleware.Tracing,         // request/session IDs (enriches logger)
//	    middleware.MetricsMonitor,  // Prometheus metrics
//	    middleware.ContentTypeJson, // JSON content type
//	    middleware.Cors(cfg),       // CORS headers, OPTIONS short-circuit
//	)
//
// The /health endpoint is automatically excluded from logging and metrics.
package middleware

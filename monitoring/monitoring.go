package monitoring

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

// meterName is the instrumentation scope under which garlic's HTTP metrics are
// recorded. It identifies the source of the metrics on the exported telemetry.
const meterName = "github.com/luanguimaraesla/garlic"

// durationBuckets are the explicit histogram boundaries (in seconds) for
// http.server.request.duration. They match the OpenTelemetry HTTP semantic
// convention recommendation.
var durationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
}

// instruments holds the OpenTelemetry instruments mirroring the HTTP metrics
// garlic records for every request.
type instruments struct {
	requests       metric.Int64Counter
	activeRequests metric.Int64UpDownCounter
	duration       metric.Float64Histogram
}

// getInstruments lazily builds the instruments from the global MeterProvider on
// first use. Deferring creation until the first request ensures the instruments
// bind to whatever MeterProvider the application installed at startup, rather
// than the no-op provider present at package import time.
var getInstruments = sync.OnceValues(func() (*instruments, error) {
	meter := otel.Meter(meterName)

	requests, err := meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total number of HTTP requests."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, logInstrumentError(err, "http.server.requests counter")
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of in-flight HTTP requests."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, logInstrumentError(err, "http.server.active_requests counter")
	}

	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	if err != nil {
		return nil, logInstrumentError(err, "http.server.request.duration histogram")
	}

	return &instruments{
		requests:       requests,
		activeRequests: activeRequests,
		duration:       duration,
	}, nil
})

// logInstrumentError propagates and logs an instrument-construction failure.
// It runs inside the sync.OnceValues body, so the failure is logged exactly
// once rather than on every request.
func logInstrumentError(err error, name string) error {
	gerr := errors.Propagate(err, "failed to create "+name)
	logging.Global().Error("Failed to initialize HTTP metrics", errors.Zap(gerr))
	return gerr
}

// resolve returns the lazily built instruments, or nil if construction failed
// (the failure was already logged once). Callers no-op when nil so a metrics
// failure never breaks request handling.
func resolve() *instruments {
	inst, _ := getInstruments()
	return inst
}

// IncrementTraffic records a completed HTTP request on the
// http.server.requests counter.
func IncrementTraffic(ctx context.Context, method, route string, status int) {
	inst := resolve()
	if inst == nil {
		return
	}

	inst.requests.Add(ctx, 1, metric.WithAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute(route),
		semconv.HTTPResponseStatusCode(status),
	))
}

// IncrementActiveRequests records the start of an HTTP request on the
// http.server.active_requests up/down counter.
func IncrementActiveRequests(ctx context.Context, method, route string) {
	inst := resolve()
	if inst == nil {
		return
	}

	inst.activeRequests.Add(ctx, 1, metric.WithAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute(route),
	))
}

// DecrementActiveRequests records the end of an HTTP request on the
// http.server.active_requests up/down counter.
func DecrementActiveRequests(ctx context.Context, method, route string) {
	inst := resolve()
	if inst == nil {
		return
	}

	inst.activeRequests.Add(ctx, -1, metric.WithAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute(route),
	))
}

// ObserveLatency records the duration in seconds of a completed HTTP request on
// the http.server.request.duration histogram.
func ObserveLatency(ctx context.Context, method, route string, status int, seconds float64) {
	inst := resolve()
	if inst == nil {
		return
	}

	inst.duration.Record(ctx, seconds, metric.WithAttributes(
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute(route),
		semconv.HTTPResponseStatusCode(status),
	))
}

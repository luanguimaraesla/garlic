//go:build unit
// +build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// reader is the manual reader installed for the whole test binary. The metrics
// instruments in the monitoring package bind to this provider the first time
// they are recorded, so it must be the only MeterProvider set here.
var reader *sdkmetric.ManualReader

func TestMain(m *testing.M) {
	reader = sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	os.Exit(m.Run())
}

func okHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			panic(err)
		}
	}
}

// collect gathers the current cumulative metrics from the manual reader.
func collect(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	return rm
}

// attrsMatch reports whether the data point attribute set is exactly the given
// key/value pairs.
func attrsMatch(set attribute.Set, kvs ...attribute.KeyValue) bool {
	if set.Len() != len(kvs) {
		return false
	}

	for _, kv := range kvs {
		value, ok := set.Value(kv.Key)
		if !ok || value != kv.Value {
			return false
		}
	}

	return true
}

// findAggregation returns the data recorded under the instrument name, or nil
// if it has not been recorded yet.
func findAggregation(rm metricdata.ResourceMetrics, name string) metricdata.Aggregation {
	for _, sm := range rm.ScopeMetrics {
		for _, metric := range sm.Metrics {
			if metric.Name == name {
				return metric.Data
			}
		}
	}

	return nil
}

// counterValue returns the current cumulative value of the int64 sum data point
// matching the attributes, or 0 if absent. Reads against a baseline keep the
// assertions independent of other tests and repeated runs.
func counterValue(t *testing.T, name string, kvs ...attribute.KeyValue) int64 {
	t.Helper()

	sum, ok := findAggregation(collect(t), name).(metricdata.Sum[int64])
	if !ok {
		return 0
	}

	for _, dp := range sum.DataPoints {
		if attrsMatch(dp.Attributes, kvs...) {
			return dp.Value
		}
	}

	return 0
}

// histogramPoint returns the float64 histogram data point matching the
// attributes, or a zero point with found=false if absent.
func histogramPoint(t *testing.T, name string, kvs ...attribute.KeyValue) (metricdata.HistogramDataPoint[float64], bool) {
	t.Helper()

	hist, ok := findAggregation(collect(t), name).(metricdata.Histogram[float64])
	if !ok {
		return metricdata.HistogramDataPoint[float64]{}, false
	}

	for _, dp := range hist.DataPoints {
		if attrsMatch(dp.Attributes, kvs...) {
			return dp, true
		}
	}

	return metricdata.HistogramDataPoint[float64]{}, false
}

// bucketCount returns the count in the bucket with the given upper bound.
func bucketCount(point metricdata.HistogramDataPoint[float64], upper float64) uint64 {
	for i, b := range point.Bounds {
		if b == upper {
			return point.BucketCounts[i]
		}
	}

	return 0
}

func TestTrafficMonitoring(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(http.MethodGet),
		semconv.HTTPRoute("unknown"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	}

	before := counterValue(t, "http.server.requests", attrs...)

	rec := httptest.NewRecorder()
	MetricsMonitor(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(1), counterValue(t, "http.server.requests", attrs...)-before)
}

func TestActiveRequestsMonitoring(t *testing.T) {
	// A distinct method keeps this test's data point disjoint from the others,
	// since route ("unknown") and status (200) are shared.
	method := http.MethodPost
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
	}

	before := counterValue(t, "http.server.active_requests", attrs...)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inside the handler the request is in flight: the up/down counter
		// must read one above the baseline. ManualReader.Collect is synchronous.
		during := counterValue(t, "http.server.active_requests", attrs...)
		assert.Equal(t, before+1, during, "active requests should increase during the request")

		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	MetricsMonitor(handler).ServeHTTP(rec, httptest.NewRequest(method, "/test", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, before, counterValue(t, "http.server.active_requests", attrs...),
		"active requests should return to the baseline after the request")
}

func TestLatencyMonitoring(t *testing.T) {
	method := http.MethodPut
	requestRunTime := 1.1
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	}

	before, _ := histogramPoint(t, "http.server.request.duration", attrs...)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(requestRunTime * float64(time.Second)))
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	MetricsMonitor(handler).ServeHTTP(rec, httptest.NewRequest(method, "/test", nil))

	assert.Equal(t, http.StatusOK, rec.Code)

	after, found := histogramPoint(t, "http.server.request.duration", attrs...)
	require.True(t, found, "no http.server.request.duration data point recorded")
	assert.Equal(t, uint64(1), after.Count-before.Count)

	// A 1.1s request falls in the (1, 2.5] bucket. OTEL bucket counts are
	// non-cumulative, so exactly that bucket gains the single observation.
	assert.Equal(t, uint64(1), bucketCount(after, 2.5)-bucketCount(before, 2.5))
}

func TestMetricsRecordedWhenHandlerPanics(t *testing.T) {
	method := http.MethodDelete
	activeAttrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
	}
	trafficAttrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	}

	activeBefore := counterValue(t, "http.server.active_requests", activeAttrs...)
	trafficBefore := counterValue(t, "http.server.requests", trafficAttrs...)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	require.Panics(t, func() {
		MetricsMonitor(handler).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/test", nil))
	})

	assert.Equal(t, activeBefore, counterValue(t, "http.server.active_requests", activeAttrs...),
		"active requests must return to the baseline even when the handler panics")
	assert.Equal(t, int64(1), counterValue(t, "http.server.requests", trafficAttrs...)-trafficBefore,
		"a panicking request must still be counted")
}

func TestHealthRouteSkipped(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(http.MethodGet),
		semconv.HTTPRoute("/health"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	}

	before := counterValue(t, "http.server.requests", attrs...)

	// A chi router resolves the route pattern to "/health", which the
	// middleware excludes from metrics.
	router := chi.NewRouter()
	router.Use(MetricsMonitor)
	router.Get("/health", okHandler())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(0), counterValue(t, "http.server.requests", attrs...)-before,
		"/health must be excluded from metrics")
}

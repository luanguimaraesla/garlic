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

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			panic(err)
		}
	})
}

// collect gathers the current metrics from the manual reader.
func collect(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	return rm
}

// findMetric returns the aggregation recorded under the given instrument name.
func findMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Aggregation {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, metric := range sm.Metrics {
			if metric.Name == name {
				return metric.Data
			}
		}
	}

	t.Fatalf("metric %q not found", name)
	return nil
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

// sumValue returns the int64 sum data point matching the attributes.
func sumValue(t *testing.T, data metricdata.Aggregation, kvs ...attribute.KeyValue) (int64, bool) {
	t.Helper()

	sum, ok := data.(metricdata.Sum[int64])
	require.True(t, ok, "expected Sum[int64], got %T", data)

	for _, dp := range sum.DataPoints {
		if attrsMatch(dp.Attributes, kvs...) {
			return dp.Value, true
		}
	}

	return 0, false
}

// histogramPoint returns the float64 histogram data point matching the attributes.
func histogramPoint(t *testing.T, data metricdata.Aggregation, kvs ...attribute.KeyValue) (metricdata.HistogramDataPoint[float64], bool) {
	t.Helper()

	hist, ok := data.(metricdata.Histogram[float64])
	require.True(t, ok, "expected Histogram[float64], got %T", data)

	for _, dp := range hist.DataPoints {
		if attrsMatch(dp.Attributes, kvs...) {
			return dp, true
		}
	}

	return metricdata.HistogramDataPoint[float64]{}, false
}

func TestTrafficMonitoring(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	MetricsMonitor(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	data := findMetric(t, collect(t), "http.server.requests")
	value, found := sumValue(t, data,
		semconv.HTTPRequestMethodKey.String(http.MethodGet),
		semconv.HTTPRoute("unknown"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	)

	require.True(t, found, "no http.server.requests data point for GET/unknown/200")
	assert.Equal(t, int64(1), value)
}

func TestActiveRequestsMonitoring(t *testing.T) {
	// A distinct method keeps this test's data point disjoint from the others,
	// since route ("unknown") and status (200) are shared.
	method := http.MethodPost
	req := httptest.NewRequest(method, "/test", nil)
	rec := httptest.NewRecorder()

	activeAttrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inside the handler the request is in flight: the up/down counter
		// must read 1. ManualReader.Collect is synchronous.
		data := findMetric(t, collect(t), "http.server.active_requests")
		value, found := sumValue(t, data, activeAttrs...)

		require.True(t, found, "no http.server.active_requests data point during request")
		assert.Equal(t, int64(1), value)

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			panic(err)
		}
	})

	MetricsMonitor(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// After the request completes the counter must return to 0.
	data := findMetric(t, collect(t), "http.server.active_requests")
	value, found := sumValue(t, data, activeAttrs...)

	require.True(t, found, "no http.server.active_requests data point after request")
	assert.Equal(t, int64(0), value)
}

func TestLatencyMonitoring(t *testing.T) {
	method := http.MethodPut
	requestRunTime := 1.1

	req := httptest.NewRequest(method, "/test", nil)
	rec := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(requestRunTime * float64(time.Second)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			panic(err)
		}
	})

	MetricsMonitor(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	data := findMetric(t, collect(t), "http.server.request.duration")
	point, found := histogramPoint(t, data,
		semconv.HTTPRequestMethodKey.String(method),
		semconv.HTTPRoute("unknown"),
		semconv.HTTPResponseStatusCode(http.StatusOK),
	)

	require.True(t, found, "no http.server.request.duration data point for PUT/unknown/200")
	assert.Equal(t, uint64(1), point.Count)
	assert.GreaterOrEqual(t, point.Sum, requestRunTime)

	// A 1.1s request falls in the (1, 2.5] bucket. OTEL bucket counts are
	// non-cumulative, so exactly that bucket holds the single observation.
	idx := bucketIndex(point.Bounds, 2.5)
	require.GreaterOrEqual(t, idx, 0, "2.5s boundary missing from histogram bounds")
	assert.Equal(t, uint64(1), point.BucketCounts[idx])
}

// bucketIndex returns the index of the given upper bound in the bounds slice,
// or -1 if absent.
func bucketIndex(bounds []float64, upper float64) int {
	for i, b := range bounds {
		if b == upper {
			return i
		}
	}

	return -1
}

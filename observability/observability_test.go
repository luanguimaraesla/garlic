//go:build unit
// +build unit

package observability

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	cmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
)

// captureServer is a minimal OTLP/gRPC metrics collector that records every
// export it receives.
type captureServer struct {
	cmetrics.UnimplementedMetricsServiceServer

	mu       sync.Mutex
	received []*cmetrics.ExportMetricsServiceRequest
}

func (s *captureServer) Export(_ context.Context, req *cmetrics.ExportMetricsServiceRequest) (*cmetrics.ExportMetricsServiceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.received = append(s.received, req)
	return &cmetrics.ExportMetricsServiceResponse{}, nil
}

func (s *captureServer) names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []string
	for _, req := range s.received {
		for _, rm := range req.GetResourceMetrics() {
			for _, sm := range rm.GetScopeMetrics() {
				for _, m := range sm.GetMetrics() {
					out = append(out, m.GetName())
				}
			}
		}
	}
	return out
}

func TestObservabilityPushesToCollector(t *testing.T) {
	// Start an in-process OTLP/gRPC collector.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	collector := &captureServer{}
	srv := grpc.NewServer()
	cmetrics.RegisterMetricsServiceServer(srv, collector)

	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	// Point the exporter at the collector via explicit Config overrides.
	Init(&Config{
		ServiceName: "observability-test",
		Endpoint:    lis.Addr().String(),
		Insecure:    true,
		Interval:    time.Hour, // never auto-export; we force a flush below
	})

	counter, err := otel.Meter("test").Int64Counter("test.counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 1)

	// Shutdown forces a final flush, pushing the recorded metric.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, Shutdown(ctx))

	assert.Contains(t, collector.names(), "test.counter")
}

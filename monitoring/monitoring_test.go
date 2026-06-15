//go:build unit
// +build unit

package monitoring

import (
	"context"
	"sync"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

// TestHelpersNoOpWhenInstrumentsFail verifies that the recording helpers do not
// panic when instrument construction fails and resolve returns nil.
func TestHelpersNoOpWhenInstrumentsFail(t *testing.T) {
	original := getInstruments
	t.Cleanup(func() { getInstruments = original })

	getInstruments = sync.OnceValues(func() (*instruments, error) {
		return nil, errors.New(errors.KindSystemError, "instrument construction failed")
	})

	ctx := context.Background()
	IncrementTraffic(ctx, "GET", "/x", 200)
	IncrementActiveRequests(ctx, "GET", "/x")
	DecrementActiveRequests(ctx, "GET", "/x")
	ObserveLatency(ctx, "GET", "/x", 200, 0.1)
}

//go:build unit
// +build unit

package request

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/logging"
)

func WithLogger(r *http.Request, logger *zap.Logger) *http.Request {
	ctx := context.WithValue(r.Context(), logging.LoggerKey, logger)
	return r.WithContext(ctx)
}

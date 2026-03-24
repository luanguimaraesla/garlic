package request

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/logging"
)

// GetLogger returns the logger from the given request's context
func GetLogger(r *http.Request) *zap.Logger {
	ctx := r.Context()
	return logging.GetLoggerFromContext(ctx)
}

// SetLogger associates a zap.Logger instance with the given HTTP request by
// storing it in the request's context. This allows the logger to be retrieved
// and used in subsequent processing of the request, ensuring consistent logging
// throughout the request's lifecycle.
func SetLogger(r *http.Request, logger *zap.Logger) *http.Request {
	ctx := logging.SetContextLogger(r.Context(), logger)
	return r.WithContext(ctx)
}

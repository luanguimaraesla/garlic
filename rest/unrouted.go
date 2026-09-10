package rest

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

// notFoundHandler answers a request whose path matches no route with the
// canonical error DTO, so an unrouted 404 reads like every other garlic error
// instead of chi's bare text page.
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	logUnrouted(r, "No route matches the request path.")

	WriteError(errors.Mirror(
		errors.KindForStatus(http.StatusNotFound),
		errors.Hint("No route matches the requested path."),
	)).Must(w)
}

// methodNotAllowedHandler answers a method mismatch with the canonical error
// DTO. No Allow header is sent: chi keeps the method set it computed private
// once a custom handler is installed, and garlic deliberately does not rebuild
// it.
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	logUnrouted(r, "No route matches the request method.")

	WriteError(errors.Mirror(
		errors.KindForStatus(http.StatusMethodNotAllowed),
		errors.Hint("The request method is not allowed for this path."),
	)).Must(w)
}

func logUnrouted(r *http.Request, message string) {
	unroutedLogger(r.Context()).Warn(message,
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
}

// unroutedLogger prefers the request logger the middleware installs and falls
// back to the global one, because an unrouted request can reach these handlers
// before the middleware chain, or entirely outside it when the router holds no
// routes at all.
func unroutedLogger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(logging.LoggerKey).(*zap.Logger); ok && logger != nil {
		return logger
	}

	return logging.Global()
}

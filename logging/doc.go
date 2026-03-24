// Package logging provides a singleton Zap-based structured logger with
// context integration.
//
// # Initialization
//
// Call [Init] once at application startup to configure the global logger:
//
//	logging.Init(&logging.Config{
//	    Level:    "info",
//	    Encoding: "json",
//	})
//
// If [Init] is not called, [Global] lazily initializes the logger with
// [Defaults] (level "error", encoding "json"). Calling [Init] more than once
// panics.
//
// # Usage
//
// Retrieve the singleton logger with [Global]:
//
//	logger := logging.Global()
//	logger.Info("server started", zap.String("addr", bind))
//
// # Context Integration
//
// The middleware package injects the logger into the request context.
// Retrieve it with [GetLoggerFromContext] and set it with [SetContextLogger]:
//
//	logger := logging.GetLoggerFromContext(ctx)
//	ctx = logging.SetContextLogger(ctx, logger.With(zap.String("user", id)))
package logging

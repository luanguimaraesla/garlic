// Package errors provides a rich error type with hierarchical classification,
// context propagation, and HTTP status code mapping.
//
// # Error Kinds
//
// Every error carries a [Kind] that classifies it. Kinds form a hierarchy
// rooted at [KindError]:
//
//	KindError
//	├── KindUserError (400)
//	│   ├── KindInvalidRequestError (400)
//	│   │   └── KindValidationError (400)
//	│   ├── KindNotFoundError (404)
//	│   │   └── KindDatabaseRecordNotFoundError (404)
//	│   ├── KindAuthError (401)
//	│   └── KindForbiddenError (403)
//	└── KindSystemError (500)
//	    ├── KindContextError
//	    │   └── KindContextValueNotFoundError
//	    └── KindDatabaseTransactionError (500)
//
// Each kind has a unique code, a human-readable name, an optional HTTP status
// code, and an optional parent. The [Kind.StatusCode] method traverses the
// hierarchy until it finds a defined status code.
//
// Custom kinds are registered with [Register]:
//
//	var KindRateLimitError = &errors.Kind{
//	    Name:           "RateLimitError",
//	    Code:           "E10001",
//	    Description:    "Too many requests",
//	    HTTPStatusCode: http.StatusTooManyRequests,
//	    Parent:         errors.KindUserError,
//	}
//
//	func init() { errors.Register(KindRateLimitError) }
//
// # Creating Errors
//
// Use [New] to create a fresh error:
//
//	err := errors.New(errors.KindValidationError, "email is invalid")
//
// Use [Propagate] to wrap an existing error, inheriting its kind:
//
//	err := errors.Propagate(sqlErr, "failed to fetch user")
//
// Use [PropagateAs] to wrap an existing error with an explicit kind:
//
//	err := errors.PropagateAs(errors.KindNotFoundError, sqlErr, "user not found")
//
// Both [New] and [Propagate] automatically capture a reverse trace entry.
//
// # Options
//
// Error constructors accept optional [Opt] values that enrich the error:
//
//   - [Hint] adds a user-facing suggestion included in the [DTO].
//   - [Context] captures debugging key-value pairs with automatic caller detection.
//   - [RevTrace] appends a reverse trace entry (auto-added by [New] and [Propagate]).
//   - [StackTrace] captures the full goroutine stack.
//   - [Template] creates a reusable error template (see [TemplateT]).
//
// Example with options:
//
//	err := errors.New(errors.KindSystemError, "cache miss",
//	    errors.Hint("retry the request in a few seconds"),
//	    errors.Context(errors.Field("key", cacheKey)),
//	    errors.StackTrace(),
//	)
//
// # Templates
//
// A [TemplateT] pre-configures a kind, message, and options for reuse:
//
//	var errDBTimeout = errors.Template(
//	    errors.KindDatabaseTransactionError,
//	    "database operation timed out",
//	    errors.Hint("check database connectivity"),
//	)
//
//	err := errDBTimeout.New()
//	err := errDBTimeout.Propagate(cause)
//
// # Inspection
//
// [IsKind] checks whether any error in the chain matches a kind:
//
//	if errors.IsKind(err, errors.KindNotFoundError) { ... }
//
// [AsKind] retrieves the first matching [ErrorT] in the chain:
//
//	if e, ok := errors.AsKind(err, errors.KindValidationError); ok {
//	    log.Println(e.Details)
//	}
//
// # Serialization
//
// [ErrorT.ErrorDTO] converts an error to a [DTO] suitable for JSON API
// responses. The DTO includes the kind's fully qualified name, code, message,
// and any public details.
//
// [Zap] produces a [go.uber.org/zap.Field] for structured logging that
// includes the full error context, troubleshooting data, and stack traces.
package errors

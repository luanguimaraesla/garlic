// Package errors provides a rich error type with hierarchical classification,
// context propagation, and HTTP status code mapping.
//
// # Error Kinds
//
// Every error carries a [Kind] that classifies it. Kinds form a hierarchy in
// three tiers, distinguished by a code prefix:
//
//   - Primitive kinds ("P"): the abstract roots of the hierarchy.
//   - Secondary kinds ("S"): one per standard HTTP status, named
//     "HTTP<status>Error" with code "S<status>" (e.g. HTTP404Error, S00404).
//   - Tertiary kinds ("C"): framework-specific errors that descend from the
//     primitive and secondary kinds.
//
// The hierarchy, with the secondary tier abbreviated, looks like this:
//
//	KindError (P00000)
//	├── KindUserError (P00001, 400)
//	│   └── HTTP4xxError (S004xx)                  // one per 4xx status
//	│       ├── KindInvalidRequestError (C00001, 400 <- S00400)
//	│       ├── KindAuthError (C00003, 401 <- S00401)
//	│       ├── KindForbiddenError (C00004, 403 <- S00403)
//	│       └── KindNotFoundError (C00005, 404 <- S00404)
//	└── KindSystemError (P00002, 500)
//	    └── HTTP3xxError / HTTP5xxError (S00xxx)   // one per non-4xx status
//
// The errors package defines only the primitive, secondary, and generic tertiary
// kinds above, plus shared tertiary kinds such as ContextError and
// ContextValueNotFoundError (under KindSystemError). Each garlic package owns
// and registers its own domain-specific tertiary kinds the same way: the
// validator package adds ValidationError (under KindInvalidRequestError), and
// the database package adds DatabaseRecordNotFoundError (under KindNotFoundError)
// and DatabaseTransactionError (under the 500 secondary).
//
// Each kind has a unique code, a human-readable name, an optional HTTP status
// code, and an optional parent. The [Kind.StatusCode] method traverses the
// hierarchy until it finds a defined status code, and [KindForStatus] is its
// inverse: it returns the secondary kind for any HTTP status. At init, garlic
// registers a secondary kind for every standard HTTP status; 4xx statuses are
// classified under [KindUserError] and everything else under [KindSystemError].
//
// The P, S, and C code prefixes are reserved by garlic; custom kinds must use a
// different prefix.
//
// Custom kinds are registered with [Register]:
//
//	var KindRateLimitError = &errors.Kind{
//	    Name:           "RateLimitError",
//	    Code:           "RL0001",
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
//	err := errors.New(errors.KindInvalidRequestError, "email is invalid")
//
// Use [Propagate] to wrap an existing error, inheriting its kind:
//
//	err := errors.Propagate(sqlErr, "failed to fetch user")
//
// Use [PropagateAs] to wrap an existing error with an explicit kind:
//
//	err := errors.PropagateAs(errors.KindNotFoundError, sqlErr, "user not found")
//
// [New] and every non-nil [Propagate] result automatically capture a reverse
// trace entry.
//
// # Nil Propagation
//
// [Propagate], [PropagateAs], [From], and [TemplateT.Propagate] return error.
// They return nil when the error they receive is itself a nil error interface,
// so propagating a call that succeeded stays a success:
//
//	// Returns nil when helper() succeeded.
//	return errors.Propagate(helper(), "failed to run the helper")
//
// No option runs and no error is allocated on that path. A non-nil result is
// always backed by [ErrorT]. A typed nil stored inside an error interface, such
// as a (*ErrorT)(nil) returned as error, is not equal to nil and is neither
// detected nor normalized.
//
// Use [As] or [AsKind] when code needs Kind, Details, or other members of the
// concrete *ErrorT.
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
// # Exact HTTP Statuses
//
// [ErrorT.StatusCode] is the status an error reports, and it is the status of
// the error's kind, so nothing changes for an error built the usual way.
// [Kind.CustomizeStatusCode] pins that status to an exact code, which is what a
// status received from elsewhere needs: [KindForStatus] can only classify a
// non-standard 499 or 599 as its 4xx or 5xx class, and the original code would
// otherwise be lost.
//
//	err := errors.New(
//	    errors.KindForStatus(499).CustomizeStatusCode(499),
//	    "client closed the request",
//	)
//	// errors.IsKind(err, errors.KindUserError) is true, StatusCode() is 499.
//
// What you get back is a copy of the kind, so its name, code, description,
// parent, FQN, the [DTO] it produces, and kind matching are the ones the
// original produces, and the kind it copied stays as it was, registry included.
// Propagation carries the pinned status outward, so it survives [Propagate],
// [PropagateAs], [From], and a [TemplateT]; a kind the wrapping call customized
// itself wins over the one carried from the cause. Redaction on the wire keeps
// following the kind's user or system classification, not the status class.
//
// [Kind] carries unexported state to mark a status pinned on purpose, so a kind
// declared outside this package has to use keyed fields: a positional literal,
// or a conversion from an equivalent five-field struct, no longer compiles.
// Copying a whole kind value keeps that mark, while rebuilding one from the
// exported fields alone drops it, and with it the precedence a pinned status has
// over the one a cause carries; call [Kind.CustomizeStatusCode] again to state
// that intent. Match kinds with [IsKind] or [Kind.Is] rather than comparing
// pointers with a registry entry, since a customized kind is a copy.
//
// # Templates
//
// A [TemplateT] pre-configures a kind, message, and options for reuse:
//
//	var errTimeout = errors.Template(
//	    errors.KindSystemError,
//	    "operation timed out",
//	    errors.Hint("retry the request in a few seconds"),
//	)
//
//	err := errTimeout.New()
//	err := errTimeout.Propagate(cause)
//
// # Inspection
//
// [IsKind] checks whether any error in the chain matches a kind:
//
//	if errors.IsKind(err, errors.KindNotFoundError) { ... }
//
// [AsKind] retrieves the first matching [ErrorT] in the chain:
//
//	if e, ok := errors.AsKind(err, errors.KindInvalidRequestError); ok {
//	    log.Println(e.Details)
//	}
//
// [As] recovers the concrete error regardless of kind:
//
//	var e *errors.ErrorT
//	if errors.As(err, &e) {
//	    log.Println(e.Kind().Name, e.Details)
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

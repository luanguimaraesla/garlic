// Package tracing provides request and session ID generation, storage, and
// retrieval through [context.Context].
//
// Request IDs are UUIDs generated per HTTP request. Session IDs are strings
// propagated from upstream services. Both are injected into context by the
// middleware package and accessed with:
//
//	requestID, err := tracing.GetRequestIdFromContext(ctx)
//	sessionID, err := tracing.GetSessionIdFromContext(ctx)
//
// The Must variants panic on missing values, suitable for code paths where
// middleware guarantees their presence:
//
//	requestID := tracing.MustGetRequestIdFromContext(ctx)
//	sessionID := tracing.MustGetSessionIdFromContext(ctx)
//
// Set values explicitly with [SetContextRequestId] and [SetContextSessionId].
package tracing

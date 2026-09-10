// Package httpclient provides a production HTTP client built on a
// client-as-default / request-as-fork model: a [Client] holds shared
// configuration and a single pooled http.Client, and [Client.R] forks a
// per-call [Request] that inherits the defaults and overrides them selectively.
//
// # Client
//
// Build one [Client] per upstream at startup and reuse it (the transport is
// pooled, so connections are kept alive):
//
//	conn, err := httpclient.New(&httpclient.Config{
//	    BaseURL:     "http://localhost:7233",
//	    TokenSource: httpclient.FileTokenSource("/var/run/secrets/token"),
//	    Transport:   otelhttp.NewTransport(nil), // any http.RoundTripper
//	    Retry:       httpclient.RetryConfig{Enabled: true, MaxRetries: 3},
//	})
//
// [Config] carries every shared knob; [Defaults] supplies sensible values.
// Construction follows the garlic convention (a Config struct, not functional
// options). The underlying http.Client.Timeout is intentionally left unset so a
// per-request context deadline always takes effect.
//
// # Requests
//
// [Client.R] takes the request context (mandatory: it carries the logger,
// tracing IDs, cancellation, and deadline) and returns a fluent [Request].
// Request-level setters win over client defaults:
//
//	var user User
//	_, err := conn.R(ctx).SetResult(&user).Get("/users/" + id)
//
// # Bodies
//
// SetBody is polymorphic; explicit setters give precise control. A streamed body
// with a known size sends an explicit Content-Length instead of chunked
// encoding, and large uploads are not buffered:
//
//	resp, err := conn.R(ctx).
//	    SetHeader("Content-Type", "application/octet-stream").
//	    SetBodyStream(file, stat.Size()).
//	    Put("/files/" + id + "/data")
//
// [Request.SetBodyJSON], [Request.SetBodyBytes], [Request.SetFormData], and
// [Request.SetFileReader] (multipart, streamed via io.Pipe) are also available.
//
// # Responses
//
// The returned [Response] embeds the raw http.Response. The body stays open so
// callers can decode, stream, or inspect it themselves. Close the response when
// you are done with it:
//
//	resp, err := conn.R(ctx).Get("/files/" + id + "/data")
//	defer resp.Close()
//	_, err = io.Copy(dst, resp.Body)
//
// # Auth
//
// A [TokenSource] is called fresh on every send (and every retry attempt), so a
// rotating mounted token is always current. [StaticToken] and [FileTokenSource]
// cover the common cases; a per-request token via SetAuthToken overrides it.
//
// # Retry
//
// Retry is idempotency-aware: by default only GET, HEAD, OPTIONS, PUT, and
// DELETE are retried (a streamed, non-replayable body is never retried), on
// connection errors and the retryable statuses (429, 503, 5xx except 501),
// honoring Retry-After. [Request.EnableRetry] opts a POST in; [Request.DisableRetry]
// opts out. The backoff is interruptible: cancelling the context stops it
// immediately. Supply a custom [RetryPolicy] via [RetryConfig] to change the rules.
//
// # Errors
//
// Send only returns request-building, transport, hook, and decode failures. HTTP
// statuses are left on [Response] so callers decide how to handle them. When the
// upstream returns a garlic error DTO, [Response.DecodeError] returns an error
// that works with errors.IsKind:
//
//	resp, err := conn.R(ctx).Get("/users/" + id)
//	if err != nil {
//	    return err
//	}
//	defer resp.Close()
//	if resp.IsError() {
//	    err = resp.DecodeError()
//	    if errors.IsKind(err, errors.KindNotFoundError) { /* ... */ }
//	}
//
// # Decoding an error response
//
// [Response.DecodeError] classifies the body rather than parsing it strictly. A
// body counts as a garlic error DTO when it is a JSON object carrying both core
// fields, error and a non-empty kind. Requiring the pair is what keeps foreign
// JSON that happens to have a kind discriminator out of the garlic branch, so
// its diagnostics survive instead of decoding into an empty garlic error.
//
// From there the outcome is one of three:
//
//   - A DTO whose kind is registered in this program decodes faithfully, keeping
//     the peer's kind, message, details, and origin. Fields a newer garlic added
//     are tolerated, and parsing is not size-capped, so a large valid DTO is
//     never truncated or reclassified.
//   - A DTO carrying a kind this program does not know becomes a
//     [KindUnknownResponseError] naming the received code, with the generic kind
//     for the upstream status underneath as its cause. errors.IsKind therefore
//     matches both the mismatch and the status class. This is how a kind
//     registration or garlic version mismatch between services surfaces.
//   - Anything else, from a proxy's HTML page to malformed JSON to an empty
//     body, becomes an error of the kind for the upstream status. A read that
//     dies halfway keeps the prefix that did arrive and the read failure as its
//     local cause.
//
// Every error DecodeError returns reports the exact upstream status through
// errors.ErrorT.StatusCode, including a non-standard 499 or 599 that its kind
// can only classify as a 4xx or 5xx class, so generic status matching and
// semantic kind matching both keep working. A registered DTO is no exception:
// its kind still decides classification, matching, and what the DTO says, while
// the status it reports is the one its response travelled under, the same value
// left on Response.StatusCode. That restoration is errors.DTO.DecodeFor, so a
// kind that already reports the transport status is decoded untouched.
//
// # Diagnostics kept from an error response
//
// The details of a fallback error, which is the part that crosses the wire again
// if a service writes the error back to its own caller, carry only metadata:
// http_status, content_type, body_bytes, and truncated. content_type is the bare
// media type, canonicalized and dropped when it does not parse, never the raw
// header with whatever the peer attached to it. The body itself lives in the
// troubleshooting context instead, so it reaches structured logs and nothing
// else, bounded at 4096 bytes and kept only while that bounded prefix is valid
// UTF-8; a binary body is reduced to its length.
//
// The identifiers of an unknown kind, the received code, name, and message,
// follow the same bound, each recorded with its length and a truncation marker.
// Validity is judged on the JSON as it arrived, because encoding/json rewrites
// invalid UTF-8 into U+FFFD while decoding: an identifier that arrived as binary
// keeps its received length and nothing else. The received code is also named in
// the error message, quoted, with the bound applied to the escaped form. Parser
// wording never appears in any field.
//
// None of this touches a registered DTO, which keeps the message and details the
// peer sent, in full: whether those may be forwarded is the ordinary error-class
// decision rest.WriteError makes for any garlic error.
//
// A response with a real error status and no body preserves that status rather
// than becoming a local decode error, and a malformed body is an error of the
// kind for the upstream status rather than a [KindResponseDecodeError]. The body
// is drained and closed on every path, including HEAD, an empty body, a failed
// read, and the guard that rejects a response without an error status.
//
// # Observability
//
// The connector does not import otelhttp. Compose tracing by wrapping the base
// transport, for example Config.Transport = otelhttp.NewTransport(base); tracing
// then sits inside or outside the retry loop as you choose. The existing
// X-Request-ID and X-Session-ID headers are still propagated from the context.
//
// # Extension seams
//
// Five lightweight seams keep the connector open: a custom http.RoundTripper or
// *http.Client, a [RetryPolicy], a [TokenSource], and the [BeforeRequestHook] /
// [AfterResponseHook] middleware chains.
//
// # Testing
//
// Downstream services depend on the [Requester] interface and inject the
// builder-pattern [RequesterMock] in unit tests, instead of standing up an
// httptest server.
package httpclient

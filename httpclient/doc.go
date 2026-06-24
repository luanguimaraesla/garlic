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

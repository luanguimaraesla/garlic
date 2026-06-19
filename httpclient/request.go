package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
	"github.com/luanguimaraesla/garlic/tracing"
)

// Request is a per-call fork created by [Client.R]. Its setters return the
// Request for chaining and mutate it in place; a Request is single-use and a
// verb method (Get, Post, ...) consumes it. A setter that fails records the
// error and is surfaced when the request is sent.
type Request struct {
	client *Client
	ctx    context.Context

	header        http.Header
	query         map[string]string
	body          bodyProvider
	result        any
	noParse       bool
	timeout       time.Duration
	retryOverride *bool

	before []BeforeRequestHook
	after  []AfterResponseHook

	authToken string
	basicUser string
	basicPass string
	hasBasic  bool

	buildErr error

	method string
	path   string
}

// SetHeader sets a per-request header (overriding the client base header).
func (r *Request) SetHeader(key, value string) *Request {
	r.header.Set(key, value)
	return r
}

// SetHeaders sets multiple per-request headers.
func (r *Request) SetHeaders(headers map[string]string) *Request {
	for k, v := range headers {
		r.header.Set(k, v)
	}
	return r
}

// SetQueryParam sets a single query parameter.
func (r *Request) SetQueryParam(key, value string) *Request {
	r.query[key] = value
	return r
}

// SetQueryParams sets multiple query parameters.
func (r *Request) SetQueryParams(params map[string]string) *Request {
	for k, v := range params {
		r.query[k] = v
	}
	return r
}

// SetAuthToken sets a bearer token for this request, overriding any client
// TokenSource.
func (r *Request) SetAuthToken(token string) *Request {
	r.authToken = token
	return r
}

// SetBasicAuth sets HTTP basic auth for this request.
func (r *Request) SetBasicAuth(user, password string) *Request {
	r.basicUser, r.basicPass, r.hasBasic = user, password, true
	return r
}

// SetBody sets the request body polymorphically: a []byte is sent as
// octet-stream, a string as-is, and anything else is JSON-encoded. An io.Reader
// is streamed; readers backed by an in-memory buffer (*bytes.Reader,
// *bytes.Buffer, *strings.Reader) stay replayable so idempotent requests can
// still be retried, while an arbitrary one-shot stream cannot.
func (r *Request) SetBody(v any) *Request {
	switch b := v.(type) {
	case nil:
		r.body = nil
	case []byte:
		r.body = &memoryBody{data: b, contentType: "application/octet-stream"}
	case string:
		r.body = &memoryBody{data: []byte(b)}
	case io.Reader:
		if data, ok := readableInMemory(b); ok {
			r.body = &memoryBody{data: data}
		} else {
			r.body = &streamBody{reader: b, size: -1}
		}
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return r.fail(errors.PropagateAs(errors.KindSystemError, err, "failed to encode JSON request body"))
		}
		r.body = &memoryBody{data: data, contentType: "application/json"}
	}
	return r
}

// SetBodyJSON JSON-encodes v as the request body with an application/json
// content type. The body is replayable on retry.
func (r *Request) SetBodyJSON(v any) *Request {
	data, err := json.Marshal(v)
	if err != nil {
		return r.fail(errors.PropagateAs(errors.KindSystemError, err, "failed to encode JSON request body"))
	}
	r.body = &memoryBody{data: data, contentType: "application/json"}
	return r
}

// SetBodyBytes sets a raw byte body with an explicit Content-Length, replayable
// on retry.
func (r *Request) SetBodyBytes(b []byte) *Request {
	r.body = &memoryBody{data: b, contentType: "application/octet-stream"}
	return r
}

// SetBodyStream streams reader as the body. A non-negative size sets an explicit
// Content-Length; a negative size uses chunked transfer encoding. A streamed
// body is single-pass and is not replayed on retry.
func (r *Request) SetBodyStream(reader io.Reader, size int64) *Request {
	r.body = &streamBody{reader: reader, size: size}
	return r
}

// SetBodyReader streams reader as the body with unknown length (chunked).
func (r *Request) SetBodyReader(reader io.Reader) *Request {
	r.body = &streamBody{reader: reader, size: -1}
	return r
}

// SetFormData sets an application/x-www-form-urlencoded body, replayable on retry.
func (r *Request) SetFormData(values map[string]string) *Request {
	r.body = formProvider(values)
	return r
}

// SetFileReader adds a streamed file part to a multipart/form-data body. The
// body is streamed through an io.Pipe and is not replayed on retry.
func (r *Request) SetFileReader(field, filename string, reader io.Reader) *Request {
	r.ensureMultipart().fields = append(r.ensureMultipart().fields,
		multipartField{field: field, filename: filename, reader: reader})
	return r
}

// SetMultipartField adds a plain text field to a multipart/form-data body.
func (r *Request) SetMultipartField(field, value string) *Request {
	r.ensureMultipart().fields = append(r.ensureMultipart().fields,
		multipartField{field: field, value: value})
	return r
}

// SetContentType overrides the request Content-Type.
func (r *Request) SetContentType(contentType string) *Request {
	r.header.Set("Content-Type", contentType)
	return r
}

// SetResult registers a pointer that the 2xx response body is JSON-decoded into.
func (r *Request) SetResult(v any) *Request {
	r.result = v
	return r
}

// SetDoNotParseResponse leaves the response body open and unparsed so the caller
// can stream it. The caller must Close the response.
func (r *Request) SetDoNotParseResponse(v bool) *Request {
	r.noParse = v
	return r
}

// SetContext replaces the request context.
func (r *Request) SetContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// SetTimeout sets a per-request deadline, overriding the client default.
func (r *Request) SetTimeout(d time.Duration) *Request {
	r.timeout = d
	return r
}

// EnableRetry forces retry on for this request, even for a non-idempotent method.
func (r *Request) EnableRetry() *Request {
	v := true
	r.retryOverride = &v
	return r
}

// DisableRetry forces retry off for this request.
func (r *Request) DisableRetry() *Request {
	v := false
	r.retryOverride = &v
	return r
}

// OnBeforeRequest appends a per-request before-request hook.
func (r *Request) OnBeforeRequest(h BeforeRequestHook) *Request {
	r.before = append(r.before, h)
	return r
}

// OnAfterResponse appends a per-request after-response hook.
func (r *Request) OnAfterResponse(h AfterResponseHook) *Request {
	r.after = append(r.after, h)
	return r
}

// Get sends a GET request to path.
func (r *Request) Get(path string) (*Response, error) { return r.Send(http.MethodGet, path) }

// Head sends a HEAD request to path.
func (r *Request) Head(path string) (*Response, error) { return r.Send(http.MethodHead, path) }

// Post sends a POST request to path.
func (r *Request) Post(path string) (*Response, error) { return r.Send(http.MethodPost, path) }

// Put sends a PUT request to path.
func (r *Request) Put(path string) (*Response, error) { return r.Send(http.MethodPut, path) }

// Patch sends a PATCH request to path.
func (r *Request) Patch(path string) (*Response, error) { return r.Send(http.MethodPatch, path) }

// Delete sends a DELETE request to path.
func (r *Request) Delete(path string) (*Response, error) { return r.Send(http.MethodDelete, path) }

// Options sends an OPTIONS request to path.
func (r *Request) Options(path string) (*Response, error) { return r.Send(http.MethodOptions, path) }

// Send issues the request for the given method and path and returns the typed
// Response. It is the terminal the verb methods delegate to.
func (r *Request) Send(method, path string) (*Response, error) {
	r.method, r.path = method, path

	target, err := buildURL(r.client.config.BaseURL, path, r.query)
	if err != nil {
		return nil, errors.PropagateAs(errors.KindSystemError, err, "failed to build request URL",
			errors.Context(errors.Field("http_method", method), errors.Field("http_path", path)))
	}

	ectx := errors.Context(
		errors.Field("http_method", method),
		errors.Field("http_url", target),
	)

	if r.buildErr != nil {
		return nil, errors.Propagate(r.buildErr, "invalid request", ectx)
	}

	l := r.logger().With(ectx.Zap())

	ctx, cancel := r.contextWithTimeout()
	if cancel != nil {
		defer cancel()
	}

	res, err := r.execute(ctx, l, method, target, ectx)
	if err != nil {
		return nil, err
	}

	return r.handleResponse(res, ectx)
}

func (r *Request) logger() *zap.Logger {
	if l, ok := r.ctx.Value(logging.LoggerKey).(*zap.Logger); ok {
		return l
	}
	return logging.Global()
}

func (r *Request) contextWithTimeout() (context.Context, context.CancelFunc) {
	timeout := r.timeout
	if timeout == 0 {
		timeout = r.client.config.Timeout
	}
	if timeout <= 0 {
		return r.ctx, nil
	}
	if deadline, ok := r.ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return r.ctx, nil
	}
	return context.WithTimeout(r.ctx, timeout)
}

func (r *Request) execute(ctx context.Context, l *zap.Logger, method, target string, ectx *errors.ContextT) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, errors.PropagateAs(errors.KindSystemError, err, "failed to create HTTP request", ectx)
	}

	for k, vs := range r.client.baseHeaders {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	for k, vs := range r.header {
		httpReq.Header[k] = append([]string(nil), vs...)
	}

	replayable, err := r.applyBody(httpReq)
	if err != nil {
		return nil, errors.Propagate(err, "failed to set request body", ectx)
	}

	// Once applyBody runs, httpReq.Body is live: a multipart body has a writer
	// goroutine blocked on the pipe, and a stream body may hold an open file. Any
	// early return before the request is sent must close it, or both leak.
	abort := func(e error) (*http.Response, error) {
		if httpReq.Body != nil {
			_ = httpReq.Body.Close()
		}
		return nil, e
	}

	if err := r.runBefore(httpReq); err != nil {
		return abort(errors.Propagate(err, "before-request hook failed", ectx))
	}

	r.injectTracing(ctx, httpReq)

	callerSetAuth := httpReq.Header.Get("Authorization") != "" && r.authToken == "" && !r.hasBasic
	if err := r.injectAuth(httpReq, callerSetAuth); err != nil {
		return abort(errors.Propagate(err, "failed to inject authorization", ectx))
	}

	return r.doWithRetry(ctx, l, httpReq, replayable, callerSetAuth, ectx)
}

func (r *Request) applyBody(httpReq *http.Request) (bool, error) {
	if r.body == nil {
		return true, nil
	}

	rc, contentType, length, replayable, err := r.body.body()
	if err != nil {
		return false, err
	}

	httpReq.Body = rc
	if length >= 0 {
		httpReq.ContentLength = length
	} else {
		httpReq.ContentLength = 0 // unknown length → chunked transfer
	}
	// The provider's content type is only a default: an explicit Content-Type
	// header, including one set via SetContentType, takes precedence.
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	if replayable {
		bp := r.body
		httpReq.GetBody = func() (io.ReadCloser, error) {
			body, _, _, _, gerr := bp.body()
			return body, gerr
		}
	}

	return replayable, nil
}

func (r *Request) runBefore(httpReq *http.Request) error {
	for _, h := range r.client.config.BeforeRequest {
		if err := h(r.client, r, httpReq); err != nil {
			return err
		}
	}
	for _, h := range r.before {
		if err := h(r.client, r, httpReq); err != nil {
			return err
		}
	}
	return nil
}

func (r *Request) injectTracing(ctx context.Context, httpReq *http.Request) {
	if httpReq.Header.Get("X-Session-ID") == "" {
		if sessionID, err := tracing.GetSessionIdFromContext(ctx); err == nil {
			httpReq.Header.Set("X-Session-ID", sessionID)
		}
	}
	if httpReq.Header.Get("X-Request-ID") == "" {
		if requestID, err := tracing.GetRequestIdFromContext(ctx); err == nil {
			httpReq.Header.Set("X-Request-ID", requestID.String())
		}
	}
}

// injectAuth sets the Authorization header. Explicit per-request auth (token or
// basic) wins and is stable across attempts; otherwise a client TokenSource is
// read fresh, which is why this runs again before each retry attempt. callerSet
// reports that the caller set Authorization via a header, in which case it is
// left untouched.
func (r *Request) injectAuth(httpReq *http.Request, callerSet bool) error {
	switch {
	case r.hasBasic:
		httpReq.SetBasicAuth(r.basicUser, r.basicPass)
	case r.authToken != "":
		httpReq.Header.Set("Authorization", "Bearer "+r.authToken)
	case callerSet:
		// leave the caller-provided Authorization header alone
	case r.client.config.TokenSource != nil:
		token, err := r.client.config.TokenSource.Token()
		if err != nil {
			return errors.PropagateAs(errors.KindAuthError, err, "failed to obtain auth token")
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (r *Request) doWithRetry(ctx context.Context, l *zap.Logger, httpReq *http.Request, replayable, callerSetAuth bool, ectx *errors.ContextT) (*http.Response, error) {
	rc := r.client.config.Retry

	policy := rc.Policy
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	retryEnabled := rc.Enabled && rc.MaxRetries > 0
	idempotent := IsIdempotent(httpReq.Method)
	if r.retryOverride != nil {
		retryEnabled = *r.retryOverride && rc.MaxRetries > 0
		idempotent = idempotent || *r.retryOverride
	}
	canRetry := retryEnabled && replayable && idempotent

	var resp *http.Response
	var err error

	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if httpReq.GetBody != nil {
				body, gerr := httpReq.GetBody()
				if gerr != nil {
					return nil, errors.PropagateAs(errors.KindSystemError, gerr, "failed to rewind request body", ectx)
				}
				httpReq.Body = body
			}
			if aerr := r.injectAuth(httpReq, callerSetAuth); aerr != nil {
				return nil, errors.Propagate(aerr, "failed to inject authorization", ectx)
			}
		}

		resp, err = r.client.doer.Do(httpReq)

		if !canRetry || attempt >= rc.MaxRetries {
			break
		}

		retry, cerr := policy.CheckRetry(ctx, httpReq.Method, resp, err, attempt)
		if cerr != nil {
			return nil, errors.PropagateAs(errors.KindSystemError, cerr, "request cancelled", ectx)
		}
		if !retry {
			break
		}

		if resp != nil {
			drainAndClose(resp.Body)
		}

		wait := policy.Backoff(rc.MinWait, rc.MaxWait, attempt, resp)
		l.Warn("retrying request", zap.Int("attempt", attempt+1), zap.Duration("wait", wait))

		if !sleepCtx(ctx, wait) {
			return nil, errors.PropagateAs(errors.KindSystemError, ctx.Err(), "request cancelled during backoff", ectx)
		}
	}

	if err != nil {
		return nil, errors.PropagateAs(errors.KindSystemError, err, "failed to make request", ectx)
	}

	return resp, nil
}

func (r *Request) handleResponse(res *http.Response, ectx *errors.ContextT) (*Response, error) {
	messageField := r.client.messageField

	if r.noParse {
		wrapped := &Response{raw: res, parsed: false}

		var resultErr error
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			respErr := newResponseError(res.StatusCode, res.Header, nil, messageField)
			respErr.ErrorT = errors.Propagate(respErr.ErrorT, "bad response from external service", ectx)
			resultErr = respErr
		}

		if herr := r.runAfter(wrapped); herr != nil {
			return wrapped, errors.Propagate(herr, "after-response hook failed", ectx)
		}
		return wrapped, resultErr
	}

	raw, readErr := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if readErr != nil {
		return nil, errors.PropagateAs(errors.KindSystemError, readErr, "failed to read response body", ectx)
	}

	wrapped := &Response{raw: res, body: raw, parsed: true}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		respErr := newResponseError(res.StatusCode, res.Header, raw, messageField)
		respErr.ErrorT = errors.Propagate(respErr.ErrorT, "bad response from external service", ectx)
		if herr := r.runAfter(wrapped); herr != nil {
			return wrapped, errors.Propagate(herr, "after-response hook failed", ectx)
		}
		return wrapped, respErr
	}

	if r.result != nil {
		if err := json.Unmarshal(raw, r.result); err != nil {
			return wrapped, errors.PropagateAs(errors.KindSystemError, err, "failed to decode response body", ectx)
		}
	}

	if herr := r.runAfter(wrapped); herr != nil {
		return wrapped, errors.Propagate(herr, "after-response hook failed", ectx)
	}

	return wrapped, nil
}

func (r *Request) runAfter(resp *Response) error {
	for _, h := range r.client.config.AfterResponse {
		if err := h(r.client, r, resp); err != nil {
			return err
		}
	}
	for _, h := range r.after {
		if err := h(r.client, r, resp); err != nil {
			return err
		}
	}
	return nil
}

func (r *Request) ensureMultipart() *multipartBody {
	if mb, ok := r.body.(*multipartBody); ok {
		return mb
	}
	mb := &multipartBody{}
	r.body = mb
	return mb
}

func (r *Request) fail(err error) *Request {
	if r.buildErr == nil {
		r.buildErr = err
	}
	return r
}

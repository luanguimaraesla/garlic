package httpclient

import "net/http"

// BeforeRequestHook runs after the *http.Request is built but before auth and
// tracing injection and before the request is sent. It may mutate httpReq (for
// example to add headers). Returning an error aborts the send before any
// network I/O. Client-level hooks run before per-request hooks.
type BeforeRequestHook func(c *Client, req *Request, httpReq *http.Request) error

// AfterResponseHook runs after the final response is received and (in parsed
// mode) buffered, before the verb call returns. It may inspect or remap the
// result. Returning an error fails the call. Client-level hooks run before
// per-request hooks.
type AfterResponseHook func(c *Client, req *Request, resp *Response) error

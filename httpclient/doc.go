// Package httpclient provides HTTP client functions with exponential backoff
// retry and a [Connector] for base-URL-scoped requests.
//
// # Simple Requests
//
// Package-level functions perform requests with automatic retry (initial
// interval 100 ms, max interval 2 s, total max elapsed 1 minute):
//
//	resp, err := httpclient.Get(ctx, "https://api.example.com/users")
//	resp, err := httpclient.Post(ctx, "https://api.example.com/users", payload)
//	resp, err := httpclient.Put(ctx, url, payload)
//	resp, err := httpclient.Patch(ctx, url, payload)
//	resp, err := httpclient.Delete(ctx, url)
//
// All functions propagate X-Request-ID and X-Session-ID headers from the
// request context for distributed tracing.
//
// # Connector
//
// [Connector] wraps a base URL and provides structured request building:
//
//	conn := httpclient.NewConnector(&httpclient.Config{URL: "https://api.example.com"})
//	var user User
//	err := conn.Request(ctx, &httpclient.Request{
//	    Method:      http.MethodGet,
//	    URI:         "/users/123",
//	    QueryParams: map[string]string{"fields": "name,email"},
//	}, &user)
//
// [Connector.Request] validates the response status (200 or 201) and
// JSON-decodes the body into the result pointer.
package httpclient

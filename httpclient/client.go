package httpclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/luanguimaraesla/garlic/errors"
)

// Doer is the minimal HTTP contract satisfied by *http.Client. It is the seam
// through which a custom client or a synthetic RoundTripper is injected.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Requester is the testability seam downstream services depend on so they can
// inject a mock instead of a live client. The concrete *Client satisfies it.
type Requester interface {
	R(ctx context.Context) *Request
}

// Client holds shared configuration and a single pooled http.Client, and forks
// per-call requests via R. It is safe for concurrent use.
type Client struct {
	config      *Config
	doer        Doer
	baseHeaders http.Header
}

var _ Requester = (*Client)(nil)

// New builds a Client from config. The underlying http.Client is built once
// with a pooled transport so connections are reused, and its Timeout is left
// unset so a per-request context deadline always takes effect. A nil config
// uses Defaults.
func New(config *Config) (*Client, error) {
	if config == nil {
		config = Defaults()
	}

	ectx := errors.Context(errors.Field("base_url", config.BaseURL))

	if config.BaseURL != "" {
		if _, err := url.Parse(config.BaseURL); err != nil {
			return nil, errors.PropagateAs(errors.KindSystemError, err, "invalid base URL", ectx)
		}
	}

	doer := Doer(config.HTTPClient)
	if config.HTTPClient == nil {
		transport := config.Transport
		if transport == nil {
			transport = pooledTransport()
		}
		doer = &http.Client{
			Transport:     transport,
			CheckRedirect: config.CheckRedirect,
		}
	}

	baseHeaders := http.Header{}
	for k, v := range config.BaseHeaders {
		baseHeaders.Set(k, v)
	}

	return &Client{
		config:      config,
		doer:        doer,
		baseHeaders: baseHeaders,
	}, nil
}

// R forks a per-call Request that inherits the client's defaults. ctx is
// mandatory: it carries the logger, tracing IDs, cancellation, and deadline.
func (c *Client) R(ctx context.Context) *Request {
	return &Request{
		client: c,
		ctx:    ctx,
		header: http.Header{},
		query:  map[string]string{},
	}
}

// buildURL parses the base, joins the URI path, sets the query params, and
// returns the final URL string. An empty base treats uri as an absolute URL.
func buildURL(baseURL, uri string, params map[string]string) (string, error) {
	base := baseURL
	if base == "" {
		base = uri
		uri = ""
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	if uri != "" {
		u = u.JoinPath(uri)
	}

	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}

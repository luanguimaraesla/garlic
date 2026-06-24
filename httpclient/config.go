package httpclient

import (
	"net/http"
	"time"
)

// Config carries the shared defaults for a [Client]. Per-request setters on
// [Request] override these values.
type Config struct {
	// BaseURL is prepended to every request path. An empty BaseURL means paths
	// must be absolute URLs.
	BaseURL string `mapstructure:"url" yaml:"url"`

	// BaseHeaders are applied to every request before per-request headers (which
	// win), before auth injection, and before the tracing headers.
	BaseHeaders map[string]string `mapstructure:"headers" yaml:"headers"`

	// Timeout is the default per-request deadline, applied through the request
	// context when the caller's context has no earlier deadline. 0 disables the
	// backstop. It is never set on the underlying http.Client, so a per-call
	// context deadline always takes effect.
	Timeout time.Duration `mapstructure:"timeout" yaml:"timeout"`

	// HTTPClient, when non-nil, is used as-is. Its Timeout is intentionally left
	// to the caller; garlic relies on context deadlines instead.
	HTTPClient *http.Client `mapstructure:"-" yaml:"-"`

	// Transport is the base RoundTripper. If nil, a pooled clone of
	// http.DefaultTransport is used. Ignored when HTTPClient is non-nil. Wrap
	// here to compose otelhttp, oauth2, or mutual-TLS transports.
	Transport http.RoundTripper `mapstructure:"-" yaml:"-"`

	// CheckRedirect is passed through to the underlying http.Client to control
	// redirect handling (for example, to forbid redirects on token-bearing
	// calls). Ignored when HTTPClient is non-nil.
	CheckRedirect func(req *http.Request, via []*http.Request) error `mapstructure:"-" yaml:"-"`

	// TokenSource, when non-nil, is called fresh on every send (and every retry
	// attempt) to inject "Authorization: Bearer <token>" unless the request
	// already set an Authorization header.
	TokenSource TokenSource `mapstructure:"-" yaml:"-"`

	// Retry configures retry behavior. Defaults to enabled with three retries.
	Retry RetryConfig `mapstructure:"retry" yaml:"retry"`

	// BeforeRequest and AfterResponse are ordered middleware chains run on every
	// request; per-request hooks run after these.
	BeforeRequest []BeforeRequestHook `mapstructure:"-" yaml:"-"`
	AfterResponse []AfterResponseHook `mapstructure:"-" yaml:"-"`
}

// RetryConfig tunes the retry loop. The default policy retries only idempotent
// methods; a per-request override can force retry on or off.
type RetryConfig struct {
	Enabled    bool          `mapstructure:"enabled" yaml:"enabled"`
	MaxRetries int           `mapstructure:"max" yaml:"max"`
	MinWait    time.Duration `mapstructure:"min_wait" yaml:"min_wait"`
	MaxWait    time.Duration `mapstructure:"max_wait" yaml:"max_wait"`
	Policy     RetryPolicy   `mapstructure:"-" yaml:"-"`
}

// Defaults returns a Config with a localhost base URL, a 30s timeout backstop,
// and retry enabled with exponential backoff between 100ms and 2s.
func Defaults() *Config {
	return &Config{
		BaseURL: "http://localhost",
		Timeout: 30 * time.Second,
		Retry: RetryConfig{
			Enabled:    true,
			MaxRetries: 3,
			MinWait:    100 * time.Millisecond,
			MaxWait:    2 * time.Second,
		},
	}
}

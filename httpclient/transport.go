package httpclient

import "net/http"

// RoundTripperFunc adapts a function to an [http.RoundTripper]. It is the
// lightest seam for synthesizing responses in tests without a socket.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements [http.RoundTripper].
func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// pooledTransport returns an isolated, connection-pooling transport cloned from
// the standard library default. Cloning gives keep-alives and a private
// connection pool instead of mutating the shared global transport.
func pooledTransport() http.RoundTripper {
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		return base.Clone()
	}
	return http.DefaultTransport
}

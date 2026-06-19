package httpclient

import (
	"os"
	"strings"

	"github.com/luanguimaraesla/garlic/errors"
)

// TokenSource yields a bearer token. It is called fresh on every send (and on
// every retry attempt) so a rotating token, such as a Kubernetes-projected
// secret mounted at /var/run/secrets/token, is re-read each request. A non-nil
// error aborts the send before any network I/O.
type TokenSource interface {
	Token() (string, error)
}

// TokenSourceFunc adapts a function to a [TokenSource].
type TokenSourceFunc func() (string, error)

// Token implements [TokenSource].
func (f TokenSourceFunc) Token() (string, error) { return f() }

// StaticToken returns a [TokenSource] that always yields the same token. It is
// intended for static API keys and tests.
func StaticToken(token string) TokenSource {
	return TokenSourceFunc(func() (string, error) { return token, nil })
}

// FileTokenSource returns a [TokenSource] that reads and trims the file at path
// on every call, so a rotating mounted token is always current. Read errors are
// returned as system errors and abort the send before any network I/O.
func FileTokenSource(path string) TokenSource {
	return TokenSourceFunc(func() (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", errors.PropagateAs(errors.KindSystemError, err, "failed to read token file",
				errors.Context(errors.Field("path", path)))
		}

		return strings.TrimSpace(string(b)), nil
	})
}

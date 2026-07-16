package propagation

import (
	"io"
	"net/http"

	foreign "example.com/foreign"
	"github.com/luanguimaraesla/garlic/errors"
)

func propagated(err error) error {
	return errors.Propagate(err, "handled")
}

type reader struct{ source io.Reader }

func (r *reader) Read(p []byte) (int, error) {
	return r.source.Read(p)
}

func (r *reader) unrelated(err error) error {
	return err // want "\\[G0.01\\]"
}

type roundTripper struct{}

func (t *roundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	err := errors.New(errors.KindError, "request failed")
	return nil, err
}

func (t *roundTripper) unrelated(err error) error {
	return err // want "\\[G0.01\\]"
}

func foreignConstructor() error {
	return foreign.New() // want "\\[G0.01\\]"
}

func foreignPropagation() error {
	return foreign.Propagate() // want "\\[G0.01\\]"
}

func foreignWith() error {
	return foreign.With() // want "\\[G0.01\\]"
}

func nested() error {
	fn := func(err error) error {
		return err // want "\\[G0.01\\]"
	}
	_ = fn
	return nil
}

func naked(err error) (result error) { // want "\\[G0.01\\]"
	result = err
	return
}

func tupleHelper() (int, error) {
	return 0, nil
}

func tupleReturn() (int, error) {
	return tupleHelper() // want "\\[G0.01\\]"
}

func validTupleControl() (int, error) {
	return 0, errors.Propagate(nil, "handled")
}

func garlicConstructors() error {
	return errors.New(errors.KindError, "new")
}

func templateConstructor() error {
	return errors.Template(errors.KindError, "template").New()
}

func errorWith() error {
	return (&errors.ErrorT{}).With()
}

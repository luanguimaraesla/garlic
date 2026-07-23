package propagationfallback

import "github.com/luanguimaraesla/garlic/errors"

var errStop = errors.New(errors.KindError, "stop")

type reader struct{}

func (reader) Read(p []byte) (int, error) {
	return 0, errStop
}

func (reader) unrelated(err error) error {
	return err // want "\\[G0.01\\]"
}

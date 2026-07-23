package propagation

import (
	"fmt"

	"github.com/luanguimaraesla/garlic/errors"
	"go.uber.org/zap"
)

func bare(err error) error {
	return err // want "\\[G0.01\\]"
}

func classify(err error) error {
	return errors.Propagate(err, "classified")
}

func classified(err error) error {
	return classify(err) // want "\\[G0.02\\]"
}

func wrapped(err error) error {
	return fmt.Errorf("wrapped: %w", err) // want "\\[G0.01\\]" "\\[G0.04\\]"
}

type errorOption struct{}

func (errorOption) Error() string      { return "option" }
func (errorOption) Opt(*errors.ErrorT) {}

func newWrap(err errorOption) error {
	return errors.New(errors.KindError, "wrapped", err) // want "\\[G0.06\\]"
}

type addrErr struct{ code int }

func (*addrErr) Error() string { return "address error" }

func makeAddrErr(code int) addrErr {
	return addrErr{code: code}
}

func passAddrErr(err *addrErr) *addrErr {
	return err // want "\\[G0.01\\]"
}

func logged(logger *zap.Logger, err error) error {
	if err != nil {
		logger.Error("failed", errors.Zap(err))
		return err // want "\\[G0.01\\]" "\\[G0.07\\]"
	}
	return nil
}

func loggedPropagated(logger *zap.Logger, err error) error {
	if err != nil {
		gerr := errors.Propagate(err, "operation failed")
		logger.Error("operation failed", errors.Zap(gerr))
		return gerr
	}
	return nil
}

func loggedReassigned(logger *zap.Logger, err error) error {
	var gerr error = errors.Propagate(err, "operation failed")
	gerr = err
	if err != nil {
		logger.Error("operation failed", errors.Zap(gerr))
		return gerr // want "\\[G0.01\\]" "\\[G0.07\\]"
	}
	return nil
}

func propagatedVariable(err error) error {
	gerr := errors.Propagate(err, "handled")
	return gerr
}

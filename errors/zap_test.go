//go:build unit

package errors

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

// wrappedErr embeds an *ErrorT behind a different concrete type.
type wrappedErr struct {
	*ErrorT
}

func (w wrappedErr) Unwrap() error { return w.ErrorT }

func TestZap_unwrapsToErrorT(t *testing.T) {
	w := wrappedErr{New(KindNotFoundError, "missing")}

	field := Zap(w)

	if field.Type != zapcore.ObjectMarshalerType {
		t.Errorf("expected a structured object field for a wrapped *ErrorT, got %v", field.Type)
	}
}

func TestZap_plainErrorT(t *testing.T) {
	field := Zap(New(KindSystemError, "boom"))
	if field.Type != zapcore.ObjectMarshalerType {
		t.Errorf("expected a structured object field, got %v", field.Type)
	}
}

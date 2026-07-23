package zaperror

import (
	"github.com/luanguimaraesla/garlic/errors"
	"go.uber.org/zap"
)

func invalid(err error) zap.Field {
	_ = errors.KindError
	return zap.Error(err) // want "\\[G2.02\\]"
}

package zaperror

import (
	"github.com/luanguimaraesla/garlic/errors"
	"go.uber.org/zap"
)

func valid(err error) zap.Field {
	return errors.Zap(err)
}

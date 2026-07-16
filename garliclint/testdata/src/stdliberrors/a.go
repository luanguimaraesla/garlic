package stdliberrors

import (
	"errors" // want "\\[G0.05\\]"

	gerrors "github.com/luanguimaraesla/garlic/errors"
)

var _ = errors.New
var _ = gerrors.KindError

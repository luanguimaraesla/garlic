package tracing

import "github.com/luanguimaraesla/garlic/errors"

var (
	KindContextError              = errors.Get("ContextError")
	KindContextValueNotFoundError = errors.Get("ContextValueNotFoundError")
)

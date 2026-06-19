package tracing

import "github.com/luanguimaraesla/garlic/errors"

// Context error kinds classify failures reading values from a context. They
// descend from errors.KindSystemError; importing this package registers them
// with the errors registry.
var (
	KindContextError = &errors.Kind{
		Name:        "ContextError",
		Code:        "C00006",
		Description: "An error occurred due to a problem with the context.",
		Parent:      errors.KindSystemError,
	}

	KindContextValueNotFoundError = &errors.Kind{
		Name:        "ContextValueNotFoundError",
		Code:        "C00007",
		Description: "A required value was not found in the context.",
		Parent:      KindContextError,
	}
)

func init() {
	errors.Register(
		KindContextError,
		KindContextValueNotFoundError,
	)
}

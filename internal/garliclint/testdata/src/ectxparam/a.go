package ectxparam

import "github.com/luanguimaraesla/garlic/errors"

func invalid(ectx *errors.ContextT) { // want "\\[G1.08\\]"
	_ = ectx
}

package rawvalidator

import "github.com/luanguimaraesla/garlic/validator"

type cleanValidation struct{}

func (cleanValidation) Struct(any) error { return nil }

func valid(v cleanValidation) error {
	if err := v.Struct(struct{}{}); err != nil {
		return validator.ParseValidationErrors(err)
	}
	return nil
}

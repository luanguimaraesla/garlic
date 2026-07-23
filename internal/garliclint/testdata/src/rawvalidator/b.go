package rawvalidator

import "github.com/luanguimaraesla/garlic/validator"

func valid(v *validator.Validation) error {
	if err := v.Struct(struct{}{}); err != nil {
		return validator.ParseValidationErrors(err)
	}
	return nil
}

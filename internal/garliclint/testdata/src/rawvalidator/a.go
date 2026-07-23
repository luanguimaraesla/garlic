package rawvalidator

import (
	playground "github.com/go-playground/validator/v10"
	"github.com/luanguimaraesla/garlic/validator"
)

func invalid(v *validator.Validation) error {
	if err := v.Struct(struct{}{}); err != nil {
		return err // want "\\[G5.01\\]"
	}
	return nil
}

func invalidPlayground(v *playground.Validate) error {
	if err := v.Struct(struct{}{}); err != nil {
		return err // want "\\[G5.01\\]"
	}
	return nil
}

type localValidation struct{}

func (localValidation) Struct(any) error { return nil }

func acceptableReceiver(v localValidation) error {
	if err := v.Struct(struct{}{}); err != nil {
		return err
	}
	return nil
}

func notify(error) error { return nil }

func acceptableShadowed(v *validator.Validation) error {
	if err := v.Struct(struct{}{}); err != nil {
		if err := notify(err); err != nil {
			return err
		}
		return validator.ParseValidationErrors(err)
	}
	return nil
}

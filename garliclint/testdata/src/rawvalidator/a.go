package rawvalidator

type validation struct{}

func (validation) Struct(any) error { return nil }

func invalid(v validation) error {
	if err := v.Struct(struct{}{}); err != nil {
		return err // want "\\[G5.01\\]"
	}
	return nil
}

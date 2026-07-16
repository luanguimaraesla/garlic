package validator

type Validation struct{}

func Global() *Validation                               { return &Validation{} }
func (*Validation) Struct(any) error                    { return nil }
func (*Validation) StructCtx(any, any) error            { return nil }
func (*Validation) Var(any, string) error               { return nil }
func (*Validation) VarWithValidation(any, string) error { return nil }
func ParseValidationErrors(err error) error             { return err }

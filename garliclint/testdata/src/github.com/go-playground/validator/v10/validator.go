package validator

type Validate struct{}

func New() *Validate                    { return &Validate{} }
func (*Validate) Struct(any) error      { return nil }
func (*Validate) Var(any, string) error { return nil }

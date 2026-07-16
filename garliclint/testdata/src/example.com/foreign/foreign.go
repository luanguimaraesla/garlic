package foreign

type err string

func (e err) Error() string { return string(e) }

func New() error { return err("foreign") }

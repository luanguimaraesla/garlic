package foreign

type err string

func (e err) Error() string { return string(e) }

func New() error       { return err("foreign") }
func Propagate() error { return err("foreign") }
func With() error      { return err("foreign") }

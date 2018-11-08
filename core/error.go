package core

type phperror struct {
	e error
}

func (e phperror) Run(ctx Context) (*ZVal, error) {
	return nil, e.e
}

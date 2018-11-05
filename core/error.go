package core

type phperror struct {
	e error
}

func (e phperror) run(ctx Context) (*ZVal, error) {
	return nil, e.e
}

package core

type phperror struct {
	e error
	l *Loc
}

func (e *phperror) Run(ctx Context) (*ZVal, error) {
	return nil, e.e
}

func (e *phperror) Loc() *Loc {
	return e.l
}

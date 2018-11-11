package core

type runVariable struct {
	v ZString
	l *Loc
}

func (r *runVariable) Run(ctx Context) (*ZVal, error) {
	res, err := ctx.GetVariable(r.v)
	return res, err
}

func (r *runVariable) WriteValue(ctx Context, value *ZVal) error {
	err := ctx.SetVariable(r.v, value.Dup())
	if err != nil {
		return r.l.Error(err)
	}
	return nil
}

func (r *runVariable) Loc() *Loc {
	return r.l
}

// reference to an existing [something]
type runRef struct {
	v Runnable
	l *Loc
}

func (r *runRef) Loc() *Loc {
	return r.l
}

func (r *runRef) Run(ctx Context) (*ZVal, error) {
	z, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	// embed zval into another zval
	return z.Ref(), nil
}

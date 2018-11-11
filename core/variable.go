package core

type runVariable struct {
	v ZString
	l *Loc
}

func (r *runVariable) Run(ctx Context) (*ZVal, error) {
	return ctx.GetVariable(r.v)
}

func (r *runVariable) WriteValue(ctx Context, value *ZVal) error {
	return ctx.SetVariable(r.v, value)
}

func (r *runVariable) Loc() *Loc {
	return r.l
}

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
	return &ZVal{z}, nil
}

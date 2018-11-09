package core

type Runnable interface {
	Run(Context) (*ZVal, error)
	Loc() *Loc
}

type Writable interface {
	WriteValue(ctx Context, value *ZVal) error
}

type Callable interface {
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type Runnables []Runnable

func (r Runnables) Run(ctx Context) (l *ZVal, err error) {
	for _, v := range r {
		l, err = v.Run(ctx)
		if err != nil {
			return
		}
	}
	return
}

func (r Runnables) Loc() *Loc {
	if len(r) == 0 {
		return nil
	}
	return r[0].Loc()
}

package core

type runnable interface {
	run(Context) (*ZVal, error)
}

type Writable interface {
	WriteValue(ctx Context, value *ZVal) error
}

type Callable interface {
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type runnables []runnable

func (r runnables) run(ctx Context) (l *ZVal, err error) {
	for _, v := range r {
		l, err = v.run(ctx)
		if err != nil {
			return
		}
	}
	return
}

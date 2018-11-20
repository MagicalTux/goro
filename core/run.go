package core

import "io"

type Runnable interface {
	Run(Context) (*ZVal, error)
	Dump(io.Writer) error
	Loc() *Loc
}

type Writable interface {
	WriteValue(ctx Context, value *ZVal) error
}

type Callable interface {
	Call(ctx Context, args []*ZVal) (*ZVal, error)
}

type ObjectCallable interface {
	GetMethod(method ZString, ctx Context, args []*ZVal) (*ZVal, error)
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

func (r Runnables) Dump(w io.Writer) error {
	return r.DumpWith(w, []byte{';'})
}

func (r Runnables) DumpWith(w io.Writer, sep []byte) error {
	for _, s := range r {
		err := s.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write(sep)
		if err != nil {
			return err
		}
	}
	return nil
}

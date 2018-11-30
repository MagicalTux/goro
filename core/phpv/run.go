package phpv

import "io"

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

type RunNull struct{}

func (r RunNull) Run(ctx Context) (*ZVal, error) {
	return ZNULL.ZVal(), nil
}

func (r RunNull) Dump(w io.Writer) error {
	return nil
}

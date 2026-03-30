package phpv

import "io"

type Runnables []Runnable

func (r Runnables) Run(ctx Context) (l *ZVal, err error) {
	g := ctx.Global()
	for _, v := range r {
		l, err = v.Run(ctx)
		// After each statement, release any temporary objects (e.g., GMP results
		// that were passed to var_dump/echo but never stored in a PHP variable).
		// This mimics PHP's deterministic refcount-based freeing.
		g.DrainTempObjects()
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

package core

import "io"

type runConcat []Runnable

func (r runConcat) Run(ctx Context) (l *ZVal, err error) {
	res := ""
	var t *ZVal

	for _, v := range r {
		t, err = v.Run(ctx)
		if err != nil {
			return
		}
		res = res + t.String()
	}
	l = &ZVal{ZString(res)}
	return
}

func (r runConcat) Loc() *Loc {
	if len(r) == 0 {
		return nil
	}

	return r[0].Loc()
}

func (r runConcat) Dump(w io.Writer) error {
	return r.DumpWith(w, []byte{'.'})
}

func (r runConcat) DumpWith(w io.Writer, sep []byte) error {
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

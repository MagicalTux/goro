package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type runConcat []phpv.Runnable

func (r runConcat) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	res := ""
	var t *phpv.ZVal

	for _, v := range r {
		t, err = v.Run(ctx)
		if err != nil {
			return
		}
		t, err = t.As(ctx, phpv.ZtString)
		if err != nil {
			return
		}
		res = res + t.String()
	}
	l = phpv.ZString(res).ZVal()
	return
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

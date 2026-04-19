package compiler

import (
	"io"

	"github.com/KarpelesLab/goro/core/phpv"
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
	for i, s := range r {
		err := s.Dump(w)
		if err != nil {
			return err
		}
		// Only add separator between elements, not after the last one
		if i < len(r)-1 {
			_, err = w.Write(sep)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

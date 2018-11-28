package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type Runnables []phpv.Runnable

func (r Runnables) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
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

type runParentheses struct {
	r phpv.Runnable
}

func (r *runParentheses) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'('})
	if err != nil {
		return err
	}
	err = r.r.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runParentheses) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return r.r.Run(ctx)
}

package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

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

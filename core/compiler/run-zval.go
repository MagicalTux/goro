package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type runZVal struct {
	v phpv.Val
	l *phpv.Loc
}

func (z *runZVal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return z.v.ZVal(), nil
}

func (z *runZVal) Dump(w io.Writer) error {
	// TODO
	_, err := fmt.Fprintf(w, "%#v", z.v)
	return err
}

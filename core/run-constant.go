package core

import (
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

type runConstant struct {
	c string
	l *phpv.Loc
}

func (r *runConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.c))
	return err
}

func (r *runConstant) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	switch strings.ToLower(string(r.c)) {
	case "null":
		return phpv.ZNull{}.ZVal(), nil
	case "true":
		return phpv.ZBool(true).ZVal(), nil
	case "false":
		return phpv.ZBool(false).ZVal(), nil
	}

	z, err := ctx.Global().(*Global).GetConstant(phpv.ZString(r.c))
	if err != nil {
		return nil, err
	}

	if z == nil {
		// TODO issue warning Use of undefined constant tata - assumed 'tata' (this will throw an Error in a future version of PHP)
		return phpv.ZString(r.c).ZVal(), nil
	}
	return z, nil
}

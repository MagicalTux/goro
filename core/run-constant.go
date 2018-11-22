package core

import (
	"io"
	"strings"
)

type runConstant struct {
	c string
	l *Loc
}

func (r *runConstant) Loc() *Loc {
	return r.l
}

func (r *runConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.c))
	return err
}

func (r *runConstant) Run(ctx Context) (l *ZVal, err error) {
	switch strings.ToLower(string(r.c)) {
	case "null":
		return ZNull{}.ZVal(), nil
	case "true":
		return ZBool(true).ZVal(), nil
	case "false":
		return ZBool(false).ZVal(), nil
	}

	z, err := ctx.Global().GetConstant(ZString(r.c))
	if err != nil {
		return nil, err
	}

	if z == nil {
		// TODO issue warning Use of undefined constant tata - assumed 'tata' (this will throw an Error in a future version of PHP)
		return &ZVal{ZString(r.c)}, nil
	}
	return z, nil
}

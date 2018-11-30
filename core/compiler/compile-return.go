package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileReturn(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	i, err := c.NextItem()
	c.backup()
	if err != nil {
		return nil, err
	}

	l := i.Loc()

	if i.IsSingle(';') {
		return &runReturn{nil, l}, nil // return nothing
	}

	v, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	return &runReturn{v, l}, nil
}

type runReturn struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (r *runReturn) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ret, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	return nil, &phperr.PhpReturn{L: r.l, V: ret}
}

func (r *runReturn) Dump(w io.Writer) error {
	_, err := w.Write([]byte("return "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

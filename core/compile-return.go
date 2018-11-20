package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

func compileReturn(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	i, err := c.NextItem()
	c.backup()
	if err != nil {
		return nil, err
	}

	l := MakeLoc(i.Loc())

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
	v Runnable
	l *Loc
}

func (r *runReturn) Run(ctx Context) (*ZVal, error) {
	return r.v.Run(ctx)
}

func (r *runReturn) Loc() *Loc {
	return r.l
}

func (r *runReturn) Dump(w io.Writer) error {
	_, err := w.Write([]byte("return "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

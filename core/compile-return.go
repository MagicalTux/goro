package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type PhpReturn struct {
	l *phpv.Loc
	v *phpv.ZVal
}

func compileReturn(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	i, err := c.NextItem()
	c.backup()
	if err != nil {
		return nil, err
	}

	l := phpv.MakeLoc(i.Loc())

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
	return nil, &PhpReturn{l: r.l, v: ret}
}

func (r *runReturn) Dump(w io.Writer) error {
	_, err := w.Write([]byte("return "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

func (r *PhpReturn) Error() string {
	return "You shouldn't see this - return not caught"
}

func CatchReturn(v *phpv.ZVal, err error) (*phpv.ZVal, error) {
	if err == nil {
		return v, err
	}
	switch err := err.(type) {
	case *PhpReturn:
		return err.v, nil
	case *phpv.PhpError:
		switch err := err.Err.(type) {
		case *PhpReturn:
			return err.v, nil
		}
	}
	return v, err
}

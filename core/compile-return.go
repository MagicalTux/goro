package core

import (
	"io"

	"github.com/MagicalTux/goro/core/tokenizer"
)

type PhpReturn struct {
	l *Loc
	v *ZVal
}

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
	ret, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	return nil, &PhpReturn{l: r.l, v: ret}
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

func (r *PhpReturn) Error() string {
	return "You shouldn't see this - return not caught"
}

func CatchReturn(v *ZVal, err error) (*ZVal, error) {
	if err == nil {
		return v, err
	}
	switch err := err.(type) {
	case *PhpReturn:
		return err.v, nil
	case *PhpError:
		switch err := err.e.(type) {
		case *PhpReturn:
			return err.v, nil
		}
	}
	return v, err
}

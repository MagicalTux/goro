package compiler

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableClone struct {
	arg phpv.Runnable
	l   *phpv.Loc
}

func (r *runnableClone) Dump(w io.Writer) error {
	_, err := w.Write([]byte("clone "))
	if err != nil {
		return err
	}
	return r.arg.Dump(w)
}

func (r *runnableClone) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	v, err := r.arg.Run(ctx)
	if err != nil {
		return nil, err
	}

	if v.GetType() != phpv.ZtObject {
		return nil, errors.New("__clone method called on non-object")
	}

	obj := v.Value().(phpv.ZObject)
	obj, err = obj.Clone()
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

func compileClone(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	cl := &runnableClone{l: i.Loc()}
	cl.arg, err = compileExpr(nil, c)
	return cl, err
}

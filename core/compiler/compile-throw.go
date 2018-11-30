package compiler

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableThrow struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (r *runnableThrow) Dump(w io.Writer) error {
	_, err := w.Write([]byte("throw "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

func (r *runnableThrow) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	o, ok := v.Value().(*phpobj.ZObject)
	if !ok {
		return nil, errors.New("Can only throw objects")
	}
	return nil, &phperr.PhpThrow{o}
}

func compileThrow(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	un := &runnableThrow{l: i.Loc()}
	un.v, err = compileExpr(nil, c)
	return un, err
}

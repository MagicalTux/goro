package core

import (
	"errors"
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runnableThrow struct {
	v Runnable
	l *Loc
}

func (r *runnableThrow) Loc() *Loc {
	return r.l
}

func (r *runnableThrow) Dump(w io.Writer) error {
	_, err := w.Write([]byte("throw "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

func (r *runnableThrow) Run(ctx Context) (l *ZVal, err error) {
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	o, ok := v.Value().(*ZObject)
	if !ok {
		return nil, errors.New("Can only throw objects")
	}
	return nil, &PhpThrow{o}
}

func compileThrow(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	var err error
	un := &runnableThrow{l: MakeLoc(i.Loc())}
	un.v, err = compileExpr(nil, c)
	return un, err
}

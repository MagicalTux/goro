package core

import (
	"errors"
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runnableUnset struct {
	args Runnables
	l    *Loc
}

func (r *runnableUnset) Loc() *Loc {
	return r.l
}

func (r *runnableUnset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("unset("))
	if err != nil {
		return err
	}
	err = r.args.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableUnset) Run(ctx Context) (l *ZVal, err error) {
	for _, v := range r.args {
		if x, ok := v.(Writable); ok {
			x.WriteValue(ctx, nil)
		} else {
			return nil, errors.New("unable to unset value")
		}
	}
	return nil, nil
}

func compileUnset(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	var err error
	un := &runnableUnset{l: MakeLoc(i.Loc())}
	un.args, err = compileFuncPassedArgs(c)
	return un, err
}

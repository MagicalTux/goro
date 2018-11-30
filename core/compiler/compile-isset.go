package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableIsset struct {
	args phpv.Runnables
	l    *phpv.Loc
}

func (r *runnableIsset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("isset("))
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

func (r *runnableIsset) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	for _, v := range r.args {
		r, err := v.Run(ctx)
		if err != nil {
			return nil, err
		}
		if r.GetType() == phpv.ZtNull {
			// not set
			return phpv.ZBool(false).ZVal(), nil
		}
	}
	return phpv.ZBool(true).ZVal(), nil
}

func compileIsset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	is := &runnableIsset{l: i.Loc()}
	// TODO check ()
	is.args, err = compileFuncPassedArgs(c)
	return is, err
}

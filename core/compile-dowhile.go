package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runnableDoWhile struct {
	cond Runnable
	code Runnable
	l    *Loc
}

func (r *runnableDoWhile) Run(ctx Context) (l *ZVal, err error) {
	for {
		_, err = r.code.Run(ctx)
		if err != nil {
			return nil, err
		}

		t, err := r.cond.Run(ctx)
		if err != nil {
			return nil, err
		}
		t, err = t.As(ctx, ZtBool)
		if err != nil {
			return nil, err
		}

		if !t.Value().(ZBool) {
			break
		}
	}
	return nil, nil
}

func (r *runnableDoWhile) Loc() *Loc {
	return r.l
}

func (r *runnableDoWhile) Dump(w io.Writer) error {
	_, err := w.Write([]byte("do {"))
	err = r.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("} while ("))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	if err != nil {
		return err
	}
	return err
}

func compileDoWhile(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	var err error

	// T_DO ... T_WHILE (cond)
	r := &runnableDoWhile{l: MakeLoc(i.Loc())}

	// parse code
	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	// should be T_WHILE now
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_WHILE {
		return nil, i.Unexpected()
	}

	// parse while expression
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r.cond, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for )
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	return r, nil
}

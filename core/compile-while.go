package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runnableWhile struct {
	cond Runnable
	code Runnable
	l    *Loc
}

func (r *runnableWhile) Run(ctx Context) (l *ZVal, err error) {
	for {
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

		if r.code != nil {
			_, err = r.code.Run(ctx)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

func (r *runnableWhile) Loc() *Loc {
	return r.l
}

func (r *runnableWhile) Dump(w io.Writer) error {
	_, err := w.Write([]byte("while ("))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	if r.code == nil {
		_, err = w.Write([]byte{')', ';'})
		return err
	}
	_, err = w.Write([]byte(") {"))
	if err != nil {
		return err
	}
	err = r.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func compileWhile(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// T_WHILE (expression) ...?
	l := MakeLoc(i.Loc())

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableWhile{l: l}
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

	// check for ;
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(';') {
		return r, nil
	}
	c.backup()

	// parse code
	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	return r, nil
}

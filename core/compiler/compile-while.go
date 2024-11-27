package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableWhile struct {
	cond phpv.Runnable
	code phpv.Runnable
	l    *phpv.Loc
}

func (r *runnableWhile) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	for {
		t, err := r.cond.Run(ctx)
		if err != nil {
			return nil, err
		}
		t, err = t.As(ctx, phpv.ZtBool)
		if err != nil {
			return nil, err
		}

		if !t.Value().(phpv.ZBool) {
			break
		}

		if r.code != nil {
			_, err = r.code.Run(ctx)
			if err != nil {
				e := r.l.Error(err)
				switch br := e.Err.(type) {
				case *phperr.PhpBreak:
					if br.Intv > 1 {
						br.Intv--
						return nil, br
					}
					return nil, nil
				case *phperr.PhpContinue:
					if br.Intv > 1 {
						br.Intv--
						return nil, br
					}
				default:
					return nil, e
				}
			}
		}
	}
	return nil, nil
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

func compileWhile(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// T_WHILE (expression) ...?
	l := i.Loc()

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

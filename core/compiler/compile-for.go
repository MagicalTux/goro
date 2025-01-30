package compiler

import (
	"io"
	"strconv"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileBreak(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// check if followed by digit
	intv := int64(1)

	i, err := c.NextItem()
	if i.Type == tokenizer.T_LNUMBER {
		intv, err = strconv.ParseInt(i.Data, 0, 64)
		if err != nil {
			return nil, err
		}
		if intv <= 0 {
			return nil, c.Errorf("'break' operator accepts only positive numbers")
		}
	} else {
		c.backup()
	}

	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &phperr.PhpBreak{L: i.Loc(), Intv: phpv.ZInt(intv), Initial: phpv.ZInt(intv)}, nil
}

func compileContinue(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// check if followed by digit
	intv := int64(1)

	i, err := c.NextItem()
	if i.Type == tokenizer.T_LNUMBER {
		intv, err = strconv.ParseInt(i.Data, 0, 64)
		if err != nil {
			return nil, err
		}
		if intv <= 0 {
			return nil, c.Errorf("'continue' operator accepts only positive numbers")
		}
	} else {
		c.backup()
	}

	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &phperr.PhpContinue{L: i.Loc(), Intv: phpv.ZInt(intv), Initial: phpv.ZInt(intv)}, nil
}

type runnableFor struct {
	// for (start; cond; each) ...
	// for($i = 0; $i < 4; $i++) ...
	// also, expressions can be separated by comma
	start, cond, each phpv.Runnables

	code phpv.Runnable
	l    *phpv.Loc
}

func (r *runnableFor) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	_, err = r.start.Run(ctx)
	if err != nil {
		return nil, err
	}

	for {
		err = ctx.Tick(ctx, r.l)
		if err != nil {
			return nil, err
		}

		// execute cond
		z, err := r.cond.Run(ctx)
		if err != nil {
			return nil, err
		}
		if !z.AsBool(ctx) {
			break
		}

		if r.code != nil {
			_, err = r.code.Run(ctx)
			if err != nil {
				e := r.l.Error(ctx, err)
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

		// execute each
		_, err = r.each.Run(ctx)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r *runnableFor) Loc() *phpv.Loc {
	return r.l
}

func (r *runnableFor) Dump(w io.Writer) error {
	_, err := w.Write([]byte("for("))
	if err != nil {
		return err
	}
	err = r.start.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{';'})
	if err != nil {
		return err
	}
	err = r.cond.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{';'})
	if err != nil {
		return err
	}
	err = r.each.DumpWith(w, []byte{','})
	if err != nil {
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

func compileForSub(c compileCtx, final rune) (res phpv.Runnables, err error) {
	var r phpv.Runnable

	i, err := c.NextItem()
	if i.IsSingle(final) {
		// nothing
		return
	}

	// compile comma separated list of exprs, return as runnables
	for {
		r, err = compileExpr(i, c)
		if err != nil {
			return
		}
		res = append(res, r)

		i, err = c.NextItem()

		if i.IsSingle(final) {
			return
		}
		if i.IsSingle(';') {
			i = nil
			continue
		}
		return nil, i.Unexpected()
	}
}

func compileFor(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// T_FOREACH (expression T_AS T_VARIABLE [=> T_VARIABLE]) ...?
	l := i.Loc()

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, l.Error(c, err)
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableFor{l: l}

	r.start, err = compileForSub(c, ';')
	if err != nil {
		return nil, err
	}
	r.cond, err = compileForSub(c, ';')
	if err != nil {
		return nil, err
	}
	r.each, err = compileForSub(c, ')')
	if err != nil {
		return nil, err
	}

	// check for ;
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(';') {
		return r, nil
	}

	altForm := i.IsSingle(':')
	c.backup()

	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	if altForm {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_ENDFOR {
			return nil, i.Unexpected()
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsExpressionEnd() {
			return nil, i.Unexpected()
		}
	}

	return r, nil
}

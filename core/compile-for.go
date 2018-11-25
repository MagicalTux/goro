package core

import (
	"errors"
	"io"
	"strconv"

	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileBreak(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// check if followed by digit
	intv := int64(1)

	i, err := c.NextItem()
	if i.Type == tokenizer.T_LNUMBER {
		intv, err = strconv.ParseInt(i.Data, 0, 64)
		if err != nil {
			return nil, err
		}
		if intv <= 0 {
			return nil, errors.New("'break' operator accepts only positive numbers")
		}
	} else {
		c.backup()
	}

	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpBreak{l: MakeLoc(i.Loc()), intv: ZInt(intv)}, nil
}

func compileContinue(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// check if followed by digit
	intv := int64(1)

	i, err := c.NextItem()
	if i.Type == tokenizer.T_LNUMBER {
		intv, err = strconv.ParseInt(i.Data, 0, 64)
		if err != nil {
			return nil, err
		}
		if intv <= 0 {
			return nil, errors.New("'continue' operator accepts only positive numbers")
		}
	} else {
		c.backup()
	}

	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpContinue{l: MakeLoc(i.Loc()), intv: ZInt(intv)}, nil
}

type runnableFor struct {
	// for (start; cond; each) ...
	// for($i = 0; $i < 4; $i++) ...
	// also, expressions can be separated by comma
	start, cond, each Runnables

	code Runnable
	l    *Loc
}

func (r *runnableFor) Run(ctx Context) (l *ZVal, err error) {
	_, err = r.start.Run(ctx)
	if err != nil {
		return nil, err
	}

	for {
		// execute cond
		z, err := r.cond.Run(ctx)
		if err != nil {
			return nil, err
		}
		if !z.AsBool(ctx) {
			break
		}

		_, err = r.code.Run(ctx)
		if err != nil {
			e := r.l.Error(err)
			switch br := e.e.(type) {
			case *PhpBreak:
				if br.intv > 1 {
					br.intv -= 1
					return nil, br
				}
				return nil, nil
			case *PhpContinue:
				if br.intv > 1 {
					br.intv -= 1
					return nil, br
				}
			default:
				return nil, e
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

func (r *runnableFor) Loc() *Loc {
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

func compileForSub(c compileCtx, final rune) (res Runnables, err error) {
	var r Runnable

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

func compileFor(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// T_FOREACH (expression T_AS T_VARIABLE [=> T_VARIABLE]) ...?
	l := MakeLoc(i.Loc())

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, l.Error(err)
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
	c.backup()

	// parse code
	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	return r, nil
}

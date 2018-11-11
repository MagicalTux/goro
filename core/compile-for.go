package core

import (
	"errors"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

func compileBreak(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpError{errors.New("'break' not in the 'loop' or 'switch' context"), MakeLoc(i.Loc()), PhpBreak, 1}, nil
}

func compileContinue(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpError{errors.New("'continue' not in the 'loop' context"), MakeLoc(i.Loc()), PhpContinue, 1}, nil
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
			switch e.t {
			case PhpBreak:
				return nil, nil
			case PhpContinue:
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

func compileForSub(c *compileCtx, final rune) (res Runnables, err error) {
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

func compileFor(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
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

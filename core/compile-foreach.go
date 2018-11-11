package core

import "github.com/MagicalTux/gophp/core/tokenizer"

type runnableForeach struct {
	src  Runnable
	code Runnable
	k, v ZString
	l    *Loc
}

func (r *runnableForeach) Run(ctx Context) (l *ZVal, err error) {
	z, err := r.src.Run(ctx)
	if err != nil {
		return nil, err
	}

	it := z.NewIterator()
	if it == nil {
		return nil, nil
	}

	for {
		if !it.Valid(ctx) {
			break
		}

		if r.k != "" {
			ctx.OffsetSet(ctx, r.k.ZVal(), it.Key(ctx))
		}
		ctx.OffsetSet(ctx, r.v.ZVal(), it.Current(ctx))

		_, err := r.code.Run(ctx)
		if err != nil {
			e := r.l.Error(err)
			switch e.t {
			case PhpBreak:
				return nil, nil
			case PhpContinue:
				it.Next(ctx)
				continue
			}
			return nil, e
		}
		it.Next(ctx)
	}
	return nil, nil
}

func (r *runnableForeach) Loc() *Loc {
	return r.l
}

func compileForeach(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// T_FOREACH (expression T_AS T_VARIABLE [=> T_VARIABLE]) ...?
	l := MakeLoc(i.Loc())

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableForeach{l: l}
	r.src, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for T_AS
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_AS {
		return nil, i.Unexpected()
	}

	// check for T_VARIABLE
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_VARIABLE {
		return nil, i.Unexpected()
	}

	// store in r.k or r.v ?
	varName := ZString(i.Data[1:]) // remove $

	// check for ) or =>
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_DOUBLE_ARROW {
		// check for T_VARIABLE again
		r.k = varName

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		r.v = ZString(i.Data[1:]) // remove $

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	} else {
		r.v = varName
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

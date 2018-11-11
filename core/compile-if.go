package core

import (
	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type runnableIf struct {
	cond Runnable
	yes  Runnable
	no   Runnable
	l    *Loc
}

func (r *runnableIf) Run(ctx Context) (l *ZVal, err error) {
	t, err := r.cond.Run(ctx)
	if err != nil {
		return nil, err
	}
	t, err = t.As(ctx, ZtBool)
	if err != nil {
		return nil, err
	}

	if t.v.(ZBool) {
		return r.yes.Run(ctx)
	} else if r.no != nil {
		return r.no.Run(ctx)
	} else {
		return nil, nil
	}
}

func (r *runnableIf) Loc() *Loc {
	return r.l
}

func compileIf(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// T_IF (expression) ...? else ...?

	// parse if expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableIf{l: MakeLoc(i.Loc())}
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

	// parse expression
	r.yes, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return r, err
	}

	// check for else or elseif
	switch i.Type {
	case tokenizer.T_ELSEIF:
		r.no, err = compileIf(nil, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_ELSE:
		// parse else
		r.no, err = compileBaseSingle(nil, c)
		if err != nil {
			return nil, err
		}
	default:
		c.backup()
	}

	return r, nil
}

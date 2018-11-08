package core

import (
	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type runnableIf struct {
	cond Runnable
	yes  Runnable
	no   Runnable
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
	} else {
		return r.no.Run(ctx)
	}
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

	r := &runnableIf{}
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
	// check for else (TODO check elseif)
	if i.Type != tokenizer.T_ELSE {
		c.backup()
		return r, nil
	}

	// parse else
	r.no, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	return r, nil
}

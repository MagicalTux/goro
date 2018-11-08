package core

import (
	"errors"

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
	t, _ = t.As(ctx, ZtBool)

	if t.v.(ZBool) {
		return r.yes.Run(ctx)
	} else {
		return r.no.Run(ctx)
	}
}

func compileIf(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// T_IF (expression) ...? else ...?
	return nil, errors.New("todo compileIf")
}

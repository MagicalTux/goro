package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type runnableIf struct {
	cond runnable
	yes  runnable
	no   runnable
}

func (r *runnableIf) run(ctx Context) (l *ZVal, err error) {
	t, err := r.cond.run(ctx)
	if err != nil {
		return nil, err
	}
	t, _ = t.As(ctx, ZtBool)

	if t.v.(ZBool) {
		return r.yes.run(ctx)
	} else {
		return r.no.run(ctx)
	}
}

func compileIf(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	// T_IF (expression) ...? else ...?
	return nil, errors.New("todo compileIf")
}

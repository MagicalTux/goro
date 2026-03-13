package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func compileReturn(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	i, err := c.NextItem()
	c.backup()
	if err != nil {
		return nil, err
	}

	l := i.Loc()

	if i.IsSingle(';') {
		return &runReturn{nil, l}, nil // return nothing
	}

	v, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	return &runReturn{v, l}, nil
}

type runReturn struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (r *runReturn) isReturnExprVariableLike() bool {
	if r.v == nil {
		return false
	}
	switch r.v.(type) {
	case *runVariable, *runArrayAccess, *runObjectVar, *runClassStaticVarRef:
		return true
	}
	return false
}

func (r *runReturn) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}
	var ret *phpv.ZVal
	if r.v != nil {
		var err error
		ret, err = r.v.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		ret = phpv.ZNULL.ZVal()
	}

	// Check for "Only variable references should be returned by reference"
	if fc := ctx.Func(); fc != nil {
		if cc, ok := fc.(interface{ Callable() phpv.Callable }); ok {
			if c := cc.Callable(); c != nil {
				if rr, ok := c.(interface{ ReturnsByRef() bool }); ok && rr.ReturnsByRef() {
					if !r.isReturnExprVariableLike() && (ret == nil || !ret.IsRef()) {
						// Re-tick to restore location after expression evaluation
						ctx.Tick(ctx, r.l)
						ctx.Notice("Only variable references should be returned by reference",
							logopt.NoFuncName(true))
					}
				}
			}
		}
	}

	return nil, &phperr.PhpReturn{L: r.l, V: ret}
}

func (r *runReturn) Dump(w io.Writer) error {
	_, err := w.Write([]byte("return "))
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

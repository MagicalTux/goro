package compiler

import (
	"fmt"
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
		// bare "return;" - check if never return type (never cannot have any return)
		if fn := c.getFunc(); fn != nil && fn.returnType != nil && fn.returnType.Type() == phpv.ZtNever {
			label := "function"
			if c.getClass() != nil {
				label = "method"
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("A never-returning %s must not return", label),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
		return &runReturn{nil, l}, nil // return nothing
	}

	// Check for void return type - cannot return a value
	if fn := c.getFunc(); fn != nil && fn.returnType != nil {
		rt := fn.returnType.Type()
		if rt == phpv.ZtVoid {
			label := "function"
			if c.getClass() != nil {
				label = "method"
			}
			// Check if the return value is explicitly NULL for a better error message
			if i.Type == tokenizer.T_STRING && phpv.ZString(i.Data).ToLower() == "null" {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("A void %s must not return a value (did you mean \"return;\" instead of \"return null;\"?)", label),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("A void %s must not return a value", label),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
		if rt == phpv.ZtNever {
			label := "function"
			if c.getClass() != nil {
				label = "method"
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("A never-returning %s must not return", label),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
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

	// If returning by reference and the expression is a property access,
	// enable write context so ObjectGetQuiet is used instead of ObjectGet.
	// This suppresses "Undefined property" warnings and returns the actual
	// hash table entry (not a detached copy), matching PHP semantics where
	// returning an undefined property by reference auto-creates it.
	var returnsByRef bool
	if fc := ctx.Func(); fc != nil {
		if cc, ok := fc.(interface{ Callable() phpv.Callable }); ok {
			if c := cc.Callable(); c != nil {
				if rr, ok := c.(interface{ ReturnsByRef() bool }); ok && rr.ReturnsByRef() {
					returnsByRef = true
				}
			}
		}
	}

	if returnsByRef {
		if ov, ok := r.v.(*runObjectVar); ok {
			ov.writeContext = true
			defer func() { ov.writeContext = false }()
		}
		// Check if returning a reference to a readonly property
		if rc, ok := r.v.(phpv.ReadonlyRefChecker); ok {
			if err := rc.CheckReadonlyRef(ctx); err != nil {
				return nil, err
			}
		}
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
	if returnsByRef {
		if !r.isReturnExprVariableLike() && (ret == nil || !ret.IsRef()) {
			// Re-tick to restore location after expression evaluation
			ctx.Tick(ctx, r.l)
			ctx.Notice("Only variable references should be returned by reference",
				logopt.NoFuncName(true))
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

package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runVariable struct {
	runnableChild
	v phpv.ZString
	l *phpv.Loc
}

type runVariableRef struct {
	v         phpv.Runnable
	l         *phpv.Loc
	prepared  bool
	cachedKey phpv.Val
}

func (rv *runVariable) VarName() phpv.ZString {
	return rv.v
}

func (rv *runVariable) IsUnDefined(ctx phpv.Context) bool {
	exists, _ := ctx.OffsetExists(ctx, rv.v)
	return !exists
}

func compileRunVariableRef(i *tokenizer.Item, c compileCtx, l *phpv.Loc) (phpv.Runnable, error) {
	r := &runVariableRef{l: l}
	var err error

	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if i.Type == tokenizer.Rune('{') {
		r.v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.Rune('}') {
			return nil, i.Unexpected()
		}
	} else {
		r.v, err = compileOneExpr(i, c)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *runVariable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}

	varName := r.v.String()
	if varName == "this" && ctx.This() == nil {
		return nil, ctx.Errorf("Using $this when not in object context")
	}

	res, exists, err := ctx.OffsetCheck(ctx, r.v)
	if err != nil {
		return nil, err
	}

	if !exists {
		write := false
		switch t := r.Parent.(type) {
		case *runOperator:
			write = t.opD.write
			// Suppress warnings for the left side of ?? (null coalescing)
			if t.op == tokenizer.T_COALESCE {
				write = true
			}
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		case *runnableFunctionCall:
			// Undefined variable warnings for function call args are handled
			// in Call() which has access to parameter metadata (ref vs value).
			// Suppress warnings here for all function calls.
			write = true
		case *runnableFunctionCallRef, *runObjectFunc, *runNewObject:
			// functions that take references can be considered as "write",
			// but the param info is not available here, just assume
			// write is true, and let the functions themselves
			// check for undefined variables.
			write = true
		case *runRef:
			// &$var reference creation is a write context
			write = true
		case *runnableUnset:
			// unset() on undefined variables is silently ignored
			write = true
		}

		if !write {
			if err := ctx.Warn("Undefined variable $%s",
				varName, logopt.NoFuncName(true)); err != nil {
				return phpv.ZNULL.ZVal(), err
			}
		}
	}

	if res == nil {
		res := phpv.NewZVal(phpv.ZNULL)
		res.Name = &r.v
		return res, nil
	}

	v := res.Nude()
	v.Name = &r.v
	return v, nil
}

func (r *runVariable) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	var err error
	if value == nil {
		err = ctx.OffsetUnset(ctx, r.v)
	} else {
		err = ctx.OffsetSet(ctx, r.v, value)
	}
	if err != nil {
		// Don't wrap PhpThrow errors - they need to propagate as-is
		if _, ok := err.(*phperr.PhpThrow); ok {
			return err
		}
		return r.l.Error(ctx, err)
	}
	return nil
}

func (r *runVariable) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'$'})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(r.v))
	return err
}

func (r *runVariableRef) Dump(w io.Writer) error {
	_, err := w.Write([]byte("${"))
	if err != nil {
		return err
	}
	err = r.v.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func (r *runVariableRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	name := phpv.ZString(v.String())
	v, err = ctx.OffsetGet(ctx, v)
	if v != nil {
		v = v.Nude()
	}
	v.Name = &name
	return v, err
}

func (r *runVariableRef) PrepareWrite(ctx phpv.Context) error {
	v, err := r.v.Run(ctx)
	if err != nil {
		return err
	}
	r.prepared = true
	r.cachedKey = v.Dup()
	return nil
}

func (r *runVariableRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	var key phpv.Val
	if r.prepared {
		key = r.cachedKey
		r.prepared = false
		r.cachedKey = nil
	} else {
		v, err := r.v.Run(ctx)
		if err != nil {
			return err
		}
		key = v
	}

	var err error
	if value == nil {
		err = ctx.OffsetUnset(ctx, key)
	} else {
		err = ctx.OffsetSet(ctx, key, value)
	}
	if err != nil {
		return r.l.Error(ctx, err)
	}
	return nil
}

// reference to an existing [something]
type runRef struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (r *runRef) isVariableLike() bool {
	switch r.v.(type) {
	case *runVariable, *runArrayAccess, *runObjectVar, *runClassStaticVarRef:
		return true
	}
	return false
}

func (r *runRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	z, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}

	// For non-variable expressions (e.g. function calls), check if the result
	// is already a reference (from a ref-returning function). If not, the
	// expression cannot be referenced.
	if !r.isVariableLike() && !z.IsRef() {
		// Restore location to the =& site (function calls update global loc)
		ctx.Tick(ctx, r.l)
		if err := ctx.Notice("Only variables should be assigned by reference",
			logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
		return z, nil
	}

	ref := z.Ref()
	if acc, ok := r.v.(*runArrayAccess); ok {
		// An array element is referenced,
		// this has the side-effect of making that
		// element a reference too. For instance:
		//   $foo[0] = "x";
		//   $x = &$foo[0];
		// The element at $foo[0] is now a reference too,
		// such that var_dump($foo) will show something like
		// int(0) => &string("x")
		acc.WriteValue(ctx, ref)
	}

	// embed zval into another zval
	return ref, nil
}

func (r *runRef) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'&'})
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}

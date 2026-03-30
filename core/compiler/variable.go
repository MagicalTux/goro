package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runVariable struct {
	runnableChild
	v phpv.ZString
	l *phpv.Loc
}

type runVariableRef struct {
	runnableChild
	v            phpv.Runnable
	l            *phpv.Loc
	prepared     bool
	cachedKey    phpv.Val
	reReadKey    phpv.ZString // used for post-RHS re-read in compound assignments
	hasReReadKey bool
}

func (rv *runVariable) VarName() phpv.ZString {
	return rv.v
}

func (rv *runVariable) IsUnDefined(ctx phpv.Context) bool {
	// $this has its own special error ("Using $this when not in object context"),
	// so don't report it as an undefined variable.
	if rv.v.String() == "this" {
		return false
	}
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Using $this when not in object context")
	}

	res, exists, err := ctx.OffsetCheck(ctx, r.v)
	if err != nil {
		return nil, err
	}

	if !exists {
		write := false
		switch t := r.Parent.(type) {
		case *runOperator:
			// For assignment operators, only the LHS is in write context.
			// The RHS is always in read context, so undefined variable warnings
			// should be emitted. Mark write=true only when this variable is the LHS.
			if t.opD.write && t.a == r {
				write = true
			}
			// For compound assignments (+=, -=, .=, /=, etc.), the LHS is
			// in read+write context, so undefined variable warnings should
			// still be emitted. Compound ops have both write=true and op!=nil.
			if t.opD.write && t.opD.op != nil && t.a == r {
				write = false
			}
			// For ??=, the RHS (r.b) is a read context — don't suppress warnings
			if (t.op == tokenizer.T_COALESCE_EQUAL || t.op == tokenizer.T_COALESCE) && t.b == r {
				write = false
			}
		case *runArrayAccess:
			// Only suppress for the container variable, not the offset.
			// In $a[$c], $a is in write context but $c is in read context.
			if t.value == r {
				write = true
			}
		case *runDestructure:
			write = true
		case *runnableFunctionCall:
			// Undefined variable warnings for function call args are handled
			// in Call() which has access to parameter metadata (ref vs value).
			// Suppress warnings here for all function calls.
			write = true
		case *NamedArg:
			// Named arguments are wrapped in NamedArg; the parent of the inner
			// variable is the NamedArg, not the function call. Undefined variable
			// warnings for named args are handled in Call() just like positional args.
			write = true
		case *runnableFunctionCallRef:
			// For dynamic function calls ($foo()), don't suppress warnings
			// when this variable is the function name itself. PHP triggers
			// "Undefined variable" before trying to call it.
			// Only suppress for arguments (handled in Call()).
			if t.name != r {
				write = true
			}
		case *runNewObject:
			// For new $foo(), don't suppress the "Undefined variable" warning.
			// PHP triggers it before trying to instantiate, so the error handler
			// can convert it to an exception and the catch block catches it.
			// Only suppress for constructor arguments (handled in Call()).
			if t.cl != r {
				write = true
			}
		case *runObjectFunc:
			// For method calls ($obj->method()), don't suppress warnings
			// when this variable is the receiver (ref). PHP triggers "Undefined
			// variable" before evaluating the method call.
			// Only suppress for arguments (where ref info isn't available here).
			if t.ref != r {
				write = true
			}
		case *runObjectVar:
			// PHP 8 warns about undefined $var when used as $var->prop in READ context,
			// but NOT in write context. In write context (e.g. $null->a = 42),
			// only the "Attempt to assign/modify property" error is produced.
			// NOTE: For ++/--, compound assignment (incDecCtx, compoundWriteCtx), PHP
			// DOES emit the "Undefined variable" warning before the property error.
			// So only suppress for plain writeContext (simple assignment).
			if t.writeContext {
				write = true
			}
		case *runRef:
			// &$var reference creation is a write context
			write = true
		case *runnableUnset:
			// unset() on undefined variables is silently ignored
			write = true
		case phpv.Runnables:
			// Bare variable used as a statement ($var;) - PHP's compiler
			// optimizes this away (no FETCH_R opcode), so no warning.
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
	var name phpv.ZString
	if r.prepared {
		// Use cached key (set by PrepareWrite for container array access, or by a
		// previous Run() call for compound assignment operators). This ensures that
		// if $n was captured early (e.g., PrepareWrite captured 'a' before ++$n ran),
		// both Run() and WriteValue() use the same captured key.
		name = r.cachedKey.(phpv.ZString)
	} else {
		v, err := r.v.Run(ctx)
		if err != nil {
			return nil, err
		}
		name = phpv.ZString(v.String())

		// Store the pre-RHS key for compound write LHS ($$n .= rhs) so that
		// run-operator.go can re-read the current value of $$n after the RHS.
		// We use a SEPARATE field (reReadKey) to avoid interfering with WriteValue,
		// which should use the POST-RHS value of $n (so writes go to the current $$n).
		if op, ok := r.Parent.(*runOperator); ok {
			if op.opD != nil && op.opD.write && op.opD.op != nil && op.a == r {
				r.reReadKey = name
				r.hasReReadKey = true
			}
		}
	}

	// Check if this variable-variable is in a write context (like runVariable does)
	write := false
	if r.Parent == nil {
		// If parent is unknown (e.g., used by global statement), treat as write
		write = true
	} else {
		switch t := r.Parent.(type) {
		case *runOperator:
			write = t.opD.write
			// For compound assignments (+=, /=, .=, etc.) on runVariableRef ($$n),
			// PHP uses FETCH_W which auto-creates the slot silently, so treat as write
			// (suppress "Undefined variable" warning). This differs from runVariable ($a)
			// where compound assignments do emit the warning.
			// Note: for ??= (coalesce-equal) RHS, always suppress.
			if (t.op == tokenizer.T_COALESCE_EQUAL || t.op == tokenizer.T_COALESCE) && t.b == r {
				write = false
			}
		case *runArrayAccess, *runDestructure:
			write = true
		case *runRef:
			write = true
		case *runnableUnset:
			write = true
		case *runGlobal:
			write = true
		case phpv.Runnables:
			write = true
		}
	}

	res, exists, err := ctx.OffsetCheck(ctx, name)
	if err != nil {
		return nil, err
	}

	if !exists && !write {
		if err := ctx.Warn("Undefined variable $%s",
			name, logopt.NoFuncName(true)); err != nil {
			nv := phpv.NewZVal(phpv.ZNULL)
			nv.Name = &name
			return nv, err
		}
	}

	if res != nil {
		nv := res.Nude()
		nv.Name = &name
		return nv, nil
	}
	nv := phpv.NewZVal(phpv.ZNULL)
	nv.Name = &name
	return nv, nil
}

func (r *runVariableRef) PrepareWrite(ctx phpv.Context) error {
	// For variable variables used as ARRAY CONTAINERS ($$n[offset] = rhs),
	// we must evaluate $$n (capture the current $n) BEFORE the offset is
	// evaluated. For example, $$n[++$n] = "test" must use $n BEFORE ++$n runs.
	//
	// For variable variables used as DIRECT WRITE TARGETS ($$n = rhs),
	// do NOT pre-evaluate. PHP evaluates the write target AFTER the RHS,
	// so $$n = $$n[++$n] = "test" should write to the post-increment $n value.
	switch r.Parent.(type) {
	case *runArrayAccess, *runDestructure:
		// Container of an array access or destructure: evaluate and cache key now
		v, err := r.v.Run(ctx)
		if err != nil {
			return err
		}
		r.prepared = true
		r.cachedKey = phpv.ZString(v.String())
	}
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
	case *runVariable, *runArrayAccess, *runObjectVar, *runObjectDynVar, *runClassStaticVarRef:
		return true
	}
	return false
}

func (r *runRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Check if the inner access is a string offset — not allowed for references.
	// But first, we must let Run() proceed so that "Illegal string offset" warnings
	// fire (the error handler may convert them to exceptions before we get here).
	// Strategy: peek at the container type; if it's a string, run the full access
	// (which may emit the warning/exception), then throw "Cannot create references".
	isStringOffset := false
	if acc, ok := r.v.(*runArrayAccess); ok {
		container, containerErr := acc.value.Run(ctx)
		if containerErr == nil && container != nil && container.GetType() == phpv.ZtString {
			isStringOffset = true
		}
	}

	if isStringOffset {
		// Run the full access so "Illegal string offset" warning fires first.
		// If the error handler converts it to an exception, we return that.
		_, err := r.v.Run(ctx)
		if err != nil {
			return nil, err
		}
		// If we got here, the warning was suppressed. Still throw the reference error.
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot create references to/from string offsets")
	}

	// Reference creation is a write context — suppress "Undefined array key" warnings
	// since the element will be created by the reference.
	if acc, ok := r.v.(*runArrayAccess); ok {
		acc.SetWriteContext(true)
		defer acc.SetWriteContext(false)
	}
	// For object property references, set writeContext so that
	// "Attempt to modify property" is used instead of "Attempt to read property"
	if ov, ok := r.v.(*runObjectVar); ok {
		ov.writeContext = true
		defer func() { ov.writeContext = false }()
	}

	// Check if creating a reference would violate readonly constraints
	// (e.g., $ref = &$enum->value)
	if rc, ok := r.v.(phpv.ReadonlyRefChecker); ok {
		if err := rc.CheckReadonlyRef(ctx); err != nil {
			return nil, err
		}
	}

	// For array accesses, enable compound caching so that Run() caches the
	// container and offset. WriteValue (called below) will then use the cached
	// values and won't re-evaluate expressions like out(expr).
	// This prevents $a[] = &$a[out(expr)] from calling out(expr) twice.
	if acc, ok := r.v.(*runArrayAccess); ok {
		acc.compoundCache = true
	}

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
	} else if ov, ok := r.v.(*runObjectVar); ok {
		// An object property is referenced — make the property itself a reference.
		// For instance:
		//   $a->x0->y1 =& $a->x0;
		// The property $a->x0 is now a reference, so var_dump($a) shows
		//   ["x0"]=> &object(...)
		ov.WriteValue(ctx, ref)
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

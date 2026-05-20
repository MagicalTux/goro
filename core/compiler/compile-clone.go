package compiler

import (
	"fmt"
	"io"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

type runnableClone struct {
	arg       phpv.Runnable
	with      phpv.Runnable // optional second argument: array of properties to set
	extra     []phpv.Runnable // additional arguments (for Dump/forward compat)
	argNames  [2]string       // named argument labels for arg and with ("" = positional)
	spread    phpv.Runnable   // clone(...$arr) spread expression
	l         *phpv.Loc
}

func (r *runnableClone) Dump(w io.Writer) error {
	_, err := w.Write([]byte("\\clone("))
	if err != nil {
		return err
	}

	// Dump spread form
	if r.spread != nil {
		_, err = w.Write([]byte("..."))
		if err != nil {
			return err
		}
		err = r.spread.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(")"))
		return err
	}

	// Dump first arg
	if r.argNames[0] != "" {
		_, err = fmt.Fprintf(w, "%s: ", r.argNames[0])
		if err != nil {
			return err
		}
	}
	err = r.arg.Dump(w)
	if err != nil {
		return err
	}

	// Dump second arg (withProperties)
	if r.with != nil {
		_, err = w.Write([]byte(", "))
		if err != nil {
			return err
		}
		if r.argNames[1] != "" {
			_, err = fmt.Fprintf(w, "%s: ", r.argNames[1])
			if err != nil {
				return err
			}
		}
		err = r.with.Dump(w)
		if err != nil {
			return err
		}
	}

	// Dump extra args
	for _, e := range r.extra {
		_, err = w.Write([]byte(", "))
		if err != nil {
			return err
		}
		err = e.Dump(w)
		if err != nil {
			return err
		}
	}

	_, err = w.Write([]byte(")"))
	return err
}

// EvalCloneExt runs the extended-form `clone($x, $with)` / `clone(...$arr)`
// / named-args clone, calling into runnableClone.Run's body. Used by
// the VM's OP_CLONE_EXT so the generic OpClassConst delegation is
// replaced by a dedicated opcode + helper.
func EvalCloneExt(ctx phpv.Context, node phpv.Runnable) (*phpv.ZVal, error) {
	return node.(*runnableClone).Run(ctx)
}

// IsCloneExtNode reports whether r is a non-basic `clone(…)` (extended
// PHP 8.5+ form). Basic `clone $x` uses OP_CLONE; this routes the
// extended form through OP_CLONE_EXT.
func IsCloneExtNode(r phpv.Runnable) bool {
	c, ok := r.(*runnableClone)
	return ok && !c.CloneIsBasic()
}

func (r *runnableClone) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	// Handle spread form: clone(...$arr)
	if r.spread != nil {
		spreadVal, err := r.spread.Run(ctx)
		if err != nil {
			return nil, err
		}
		if spreadVal.GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument must be of type array, %s given", spreadVal.GetType().TypeName()))
		}
		arr := spreadVal.AsArray(ctx)
		// Extract "object" and "withProperties" from the array
		objVal, _, _ := arr.OffsetCheck(ctx, phpv.ZString("object"))
		withVal, _, _ := arr.OffsetCheck(ctx, phpv.ZString("withProperties"))
		if objVal == nil || objVal.GetType() == phpv.ZtNull {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "clone(): Argument #1 ($object) must be of type object, null given")
		}
		// Run clone with these values
		return runCloneWithValues(ctx, objVal, withVal)
	}

	v, err := r.arg.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Evaluate the withProperties argument before cloning
	var withProps *phpv.ZVal
	if r.with != nil {
		withProps, err = r.with.Run(ctx)
		if err != nil {
			return nil, err
		}
		if withProps.GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument #2 ($withProperties) must be of type array, %s given", withProps.GetType().TypeName()))
		}
	}

	return runCloneWithValues(ctx, v, withProps)
}

// EvalClone implements the basic `clone $v` operation. Shared runtime
// helper so the VM emits a single point of entry. For the PHP 8.5+
// `clone($obj, $with)` / `clone(...$arr)` forms the AST still owns
// argument evaluation (named args, spread); callers route those to
// runnableClone.Run.
func EvalClone(ctx phpv.Context, v *phpv.ZVal) (*phpv.ZVal, error) {
	return runCloneWithValues(ctx, v, nil)
}

func runCloneWithValues(ctx phpv.Context, v *phpv.ZVal, withProps *phpv.ZVal) (*phpv.ZVal, error) {
	if v.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument #1 ($object) must be of type object, %s given", v.GetType().TypeName()))
	}

	obj := v.Value().(phpv.ZObject)

	// Enums and Generators cannot be cloned
	if obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 || obj.GetClass().GetName() == "Generator" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Trying to clone an uncloneable object of class %s", obj.GetClass().GetName()))
	}

	// Check __clone visibility
	if m, ok := obj.GetClass().GetMethod("__clone"); ok {
		if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
			callerClass := ctx.Class()
			if m.Modifiers.IsPrivate() {
				if callerClass == nil || callerClass.GetName() != obj.GetClass().GetName() {
					// If __clone is a native (internal) private method, treat as uncloneable
					if _, isNative := m.Method.(phpobj.NativeMethod); isNative {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Trying to clone an uncloneable object of class %s", obj.GetClass().GetName()))
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private method %s::__clone() from global scope", obj.GetClass().GetName()))
				}
			} else {
				// protected
				if callerClass == nil || (!callerClass.InstanceOf(obj.GetClass()) && !obj.GetClass().InstanceOf(callerClass)) {
					scope := "global scope"
					if callerClass != nil {
						scope = fmt.Sprintf("scope %s", callerClass.GetName())
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected method %s::__clone() from %s", obj.GetClass().GetName(), scope))
				}
			}
		}
	}

	var err error
	obj, err = obj.Clone(ctx)
	if err != nil {
		return nil, err
	}

	// Apply withProperties: set each property on the cloned object
	if withProps != nil && withProps.GetType() == phpv.ZtArray {
		arr := withProps.AsArray(ctx)
		// Check for references in the with-properties array.
		it := arr.NewIterator()
		type refIterator interface {
			CurrentRef(phpv.Context) (*phpv.ZVal, error)
		}
		if ri, ok := it.(refIterator); ok {
			for it.Valid(ctx) {
				rv, _ := ri.CurrentRef(ctx)
				// A reference-typed entry only qualifies as an error if it
				// actually has multiple observable aliases. When the other
				// side has been unset and only this entry still points at
				// the inner ZVal, the entry behaves like a plain value and
				// clone-with should accept it (see clone_with_013.phpt
				// after the unset).
				if rv != nil && rv.IsRef() {
					inner := rv.RefTarget()
					if inner == nil || inner.AliasCount() > 1 {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot assign by reference when cloning with updated properties")
					}
				}
				it.Next(ctx)
			}
		}
		// Snapshot which readonly properties were already initialized
		// before we started applying the with-array. clone-with may
		// override readonly properties that were initialized by __clone()
		// or copied from the original, but must NOT override readonly
		// properties that become initialized *during* clone-with (for
		// example, via a set-hook on another property in the same array —
		// see clone_with_012.phpt).
		var preInitialized map[phpv.ZString]bool
		if zo, ok := obj.(*phpobj.ZObject); ok {
			preInitialized = make(map[phpv.ZString]bool)
			for k, v := range arr.Iterate(ctx) {
				_ = v
				keyStr := k.AsString(ctx)
				if zo.IsReadonlyPropertyInitialized(keyStr) {
					preInitialized[keyStr] = true
				}
			}
		}

		for k, v := range arr.Iterate(ctx) {
			keyStr := k.AsString(ctx)
			// Only unmark the readonly-init flag for properties that were
			// already initialized when clone-with started. That preserves
			// the old behavior for __clone()-initialized properties while
			// still catching "second set" from a hook firing earlier in
			// this same clone-with pass.
			if zo, ok := obj.(*phpobj.ZObject); ok && preInitialized[keyStr] {
				zo.UnmarkReadonlyInit(keyStr)
			}
			err = obj.ObjectSet(ctx, keyStr, v.ZVal())
			if err != nil {
				return nil, err
			}
		}
	}

	return obj.ZVal(), nil
}

// tryParseCloneNamedArg checks if the current position has "label:" (named argument).
// Returns (label, firstToken, isNamed, err). If it's a named arg, the ':' is consumed
// and firstToken is nil. Otherwise, firstToken holds the already-read token that
// should be passed to compileExpr (since we can't double-backup).
func tryParseCloneNamedArg(c compileCtx) (string, *tokenizer.Item, bool, error) {
	i, err := c.NextItem()
	if err != nil {
		return "", nil, false, err
	}
	if !i.IsLabel() {
		c.backup()
		return "", nil, false, nil
	}
	next, err := c.NextItem()
	if err != nil {
		return "", nil, false, err
	}
	if next.IsSingle(':') {
		return i.Data, nil, true, nil
	}
	// Not a named arg - backup only the second token, return the first for compileExpr
	c.backup()
	return "", i, false, nil
}

func compileClone(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	cl := &runnableClone{l: i.Loc()}

	// Check if clone is followed by '(' — function-call syntax: clone($obj) or clone($obj, [...])
	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if next.IsSingle('(') {
		// Parse as function-call syntax: clone(args...)
		// Check for first-class callable syntax: clone(...)
		peek, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if peek.Type == tokenizer.T_ELLIPSIS {
			// Could be clone(...) or clone(...$arr)
			closer, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if closer.IsSingle(')') {
				// clone(...) — first-class callable syntax
				return &runFirstClassCloneCallable{l: i.Loc()}, nil
			}
			// clone(...$arr) — spread syntax
			c.backup()
			spreadExpr, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			cl.spread = spreadExpr
			// Expect closing paren
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if !next.IsSingle(')') {
				return nil, next.Unexpected()
			}
			return cl, nil
		}
		c.backup()

		// Check for named first argument: clone(object: $x, ...)
		name, firstTok, isNamed, err := tryParseCloneNamedArg(c)
		if err != nil {
			return nil, err
		}
		if isNamed {
			cl.argNames[0] = name
		}

		// Parse first argument (the object expression)
		cl.arg, err = compileExpr(firstTok, c)
		if err != nil {
			return nil, err
		}

		// If the first named arg was "withProperties", swap positions
		if isNamed && name == "withProperties" {
			// This is actually the second param placed first
			cl.with = cl.arg
			cl.argNames[1] = name
			cl.argNames[0] = ""
			cl.arg = nil
		}

		// Check what follows: comma (more args) or closing paren
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if next.IsSingle(',') {
			// Check for trailing comma before closing paren
			peek, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if peek.IsSingle(')') {
				// Trailing comma: clone($obj,)
				return cl, nil
			}
			c.backup()

			// Check for named second argument
			name2, firstTok2, isNamed2, err := tryParseCloneNamedArg(c)
			if err != nil {
				return nil, err
			}

			// Parse second argument
			arg2, err := compileExpr(firstTok2, c)
			if err != nil {
				return nil, err
			}

			if isNamed2 {
				switch name2 {
				case "object":
					cl.arg = arg2
					cl.argNames[0] = name2
				case "withProperties":
					cl.with = arg2
					cl.argNames[1] = name2
				default:
					cl.with = arg2
					cl.argNames[1] = name2
				}
			} else {
				// Positional second argument
				if cl.arg == nil {
					// First arg was named withProperties, so this is the extra
					cl.arg = arg2
				} else {
					cl.with = arg2
				}
			}

			// Consume any remaining arguments
			for {
				next, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if next.IsSingle(')') {
					return cl, nil
				}
				if next.IsSingle(',') {
					// Check for trailing comma
					peek, err := c.NextItem()
					if err != nil {
						return nil, err
					}
					if peek.IsSingle(')') {
						return cl, nil
					}
					c.backup()
					// Skip additional argument expressions (check for named args)
					_, extraTok, _, _ := tryParseCloneNamedArg(c)
					extra, err := compileExpr(extraTok, c)
					if err != nil {
						return nil, err
					}
					cl.extra = append(cl.extra, extra)
					continue
				}
				return nil, next.Unexpected()
			}
		}

		if !next.IsSingle(')') {
			return nil, next.Unexpected()
		}

		return cl, nil
	}

	// Traditional syntax: clone $expr (no parens)
	c.backup()
	cl.arg, err = compileExpr(nil, c)
	return cl, err
}

// runFirstClassCloneCallable implements clone(...) first-class callable syntax.
// It resolves the clone function at runtime and wraps it in a Closure.
type runFirstClassCloneCallable struct {
	l *phpv.Loc
}

func (r *runFirstClassCloneCallable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return EvalFirstClassCloneCallable(ctx)
}

// EvalFirstClassCloneCallable returns a Closure wrapping the
// clone built-in (PHP 8.5+ `clone(...)` syntax). Shared between the
// AST runner and the VM's OP_FIRSTCLASS_CLONE.
func EvalFirstClassCloneCallable(ctx phpv.Context) (*phpv.ZVal, error) {
	f, err := ctx.Global().GetFunction(ctx, "clone")
	if err != nil {
		return nil, err
	}
	closure := phpv.Bind(f, nil)
	return phpv.NewZVal(closure), nil
}

func (r *runFirstClassCloneCallable) Dump(w io.Writer) error {
	_, err := w.Write([]byte("\\clone(...)"))
	return err
}

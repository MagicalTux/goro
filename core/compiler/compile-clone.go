package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableClone struct {
	arg  phpv.Runnable
	with phpv.Runnable // optional second argument: array of properties to set
	l    *phpv.Loc
}

func (r *runnableClone) Dump(w io.Writer) error {
	if r.with != nil {
		_, err := w.Write([]byte("\\clone("))
		if err != nil {
			return err
		}
		err = r.arg.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(", "))
		if err != nil {
			return err
		}
		err = r.with.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(")"))
		return err
	}
	_, err := w.Write([]byte("\\clone("))
	if err != nil {
		return err
	}
	err = r.arg.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(")"))
	return err
}

func (r *runnableClone) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	v, err := r.arg.Run(ctx)
	if err != nil {
		return nil, err
	}

	if v.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("clone(): Argument #1 ($object) must be of type object, %s given", v.GetType().TypeName()))
	}

	obj := v.Value().(phpv.ZObject)

	// Enums cannot be cloned
	if obj.GetClass().GetType()&phpv.ZClassTypeEnum != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Trying to clone an uncloneable object of class %s", obj.GetClass().GetName()))
	}

	// Check __clone visibility
	if m, ok := obj.GetClass().GetMethod("__clone"); ok {
		if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
			callerClass := ctx.Class()
			if m.Modifiers.IsPrivate() {
				if callerClass == nil || callerClass.GetName() != obj.GetClass().GetName() {
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

	obj, err = obj.Clone(ctx)
	if err != nil {
		return nil, err
	}

	// Apply withProperties: set each property on the cloned object
	if withProps != nil {
		arr := withProps.AsArray(ctx)
		for k, v := range arr.Iterate(ctx) {
			keyStr := k.AsString(ctx)
			err = obj.ObjectSet(ctx, keyStr, v.ZVal())
			if err != nil {
				return nil, err
			}
		}
	}

	return obj.ZVal(), nil
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
			// clone(...)  — first-class callable syntax
			closer, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if closer.IsSingle(')') {
				// Return a first-class callable that wraps clone
				return &runFirstClassCloneCallable{l: i.Loc()}, nil
			}
			// clone(...$arr) spread syntax — back up and parse normally
			c.backup()
			c.backup()
		} else {
			c.backup()
		}

		// Parse first argument (the object expression)
		cl.arg, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
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

			// Parse second argument (withProperties)
			cl.with, err = compileExpr(nil, c)
			if err != nil {
				return nil, err
			}

			// Consume any remaining arguments (they'll be ignored at compile time,
			// but PHP allows them for forward compatibility — clone($x, $a, $b, $c,))
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
					// Skip additional argument expressions
					_, err = compileExpr(nil, c)
					if err != nil {
						return nil, err
					}
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
	// Look up the clone function (registered as a built-in)
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

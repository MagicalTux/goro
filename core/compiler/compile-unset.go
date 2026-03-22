package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableUnset struct {
	args phpv.Runnables
	l    *phpv.Loc
}

func (r *runnableUnset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("unset("))
	if err != nil {
		return err
	}
	err = r.args.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableUnset) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	if r.l != nil {
		ctx.Tick(ctx, r.l)
	}
	for _, v := range r.args {
		if x, ok := v.(phpv.Writable); ok {
			// Skip reading the value for certain types:
			// - ArrayAccess: unset should only call offsetUnset, not offsetGet
			// - Object properties: reading triggers checkStaticPropertyAccess which
			//   would cause a duplicate notice (WriteValue also triggers it)
			// - Static class properties: unset always throws "Attempt to unset static property"
			//   regardless of whether the property exists, so skip the read
			_, isArrayAccess := v.(*runArrayAccess)
			_, isObjectVar := v.(*runObjectVar)
			_, isStaticVar := v.(*runClassStaticVarRef)
			if !isArrayAccess && !isObjectVar && !isStaticVar {
				zv, runErr := v.Run(ctx)
				if runErr != nil {
					return nil, runErr
				}
				if err := callDestructorIfNeeded(ctx, zv); err != nil {
					return nil, err
				}
			}
			if err := x.WriteValue(ctx, nil); err != nil {
				return nil, err
			}
		} else {
			return nil, ctx.Errorf("unable to unset value")
		}
	}
	return nil, nil
}

// callDestructorIfNeeded checks if a ZVal holds an object with __destruct,
// and if so, decrements the reference count and calls the destructor if
// the count reaches zero.
// IMPORTANT: Do NOT add recursive array traversal here. Arrays can contain
// circular references (e.g. $a = []; $a[] = &$a) which would cause an
// infinite recursion and stack overflow crash.
func callDestructorIfNeeded(ctx phpv.Context, zv *phpv.ZVal) error {
	if zv == nil || zv.GetType() != phpv.ZtObject {
		return nil
	}
	obj := zv.Value()
	if zobj, ok := obj.(phpv.ZObject); ok {
		if cls := zobj.GetClass(); cls != nil {
			if h := cls.Handlers(); h != nil && h.HandleDecRef != nil {
				h.HandleDecRef(ctx, zobj)
			}
		}
	}
	if refObj, ok := obj.(interface {
		DecRef(phpv.Context) error
	}); ok {
		return refObj.DecRef(ctx)
	}
	return nil
}

// isTemporaryExpr checks if an expression produces a temporary value that
// cannot be used in a write context (e.g., class constants, function calls).
func isTemporaryExpr(r phpv.Runnable) bool {
	switch r.(type) {
	case *runClassStaticObjRef:
		// Foo::Bar is a class constant - property access on it produces a temporary
		return true
	}
	return false
}

func compileUnset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	un := &runnableUnset{l: i.Loc()}
	un.args, err = compileFuncPassedArgs(c)
	if err != nil {
		return nil, err
	}
	// Cannot use nullsafe operator in unset()
	for _, arg := range un.args {
		if containsNullSafe(arg) {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Can't use nullsafe operator in write context"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
		// Check for temporary expression in write context
		// e.g. unset(Foo::Bar->value) where Foo::Bar is a class constant
		if ov, ok := arg.(*runObjectVar); ok {
			if isTemporaryExpr(ov.ref) {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use temporary expression in write context"),
					Code: phpv.E_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
		}
	}
	return un, nil
}

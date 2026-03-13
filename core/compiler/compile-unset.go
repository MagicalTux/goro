package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
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
	for _, v := range r.args {
		if x, ok := v.(phpv.Writable); ok {
			// Skip reading the value for certain types:
			// - ArrayAccess: unset should only call offsetUnset, not offsetGet
			// - Object properties: reading triggers checkStaticPropertyAccess which
			//   would cause a duplicate notice (WriteValue also triggers it)
			_, isArrayAccess := v.(*runArrayAccess)
			_, isObjectVar := v.(*runObjectVar)
			if !isArrayAccess && !isObjectVar {
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
// and if so, calls the destructor (with proper visibility checking).
// Returns an error if the destructor call fails (e.g., visibility error).
func callDestructorIfNeeded(ctx phpv.Context, zv *phpv.ZVal) error {
	if zv == nil || zv.GetType() != phpv.ZtObject {
		return nil
	}
	obj := zv.Value()
	zobj, ok := obj.(phpv.ZObject)
	if !ok {
		return nil
	}
	m, ok := zobj.GetClass().GetMethod("__destruct")
	if !ok {
		return nil
	}
	// Check destructor visibility
	if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if m.Modifiers.IsPrivate() {
			if callerClass == nil || callerClass.GetName() != zobj.GetClass().GetName() {
				ctx.Global().UnregisterDestructor(zobj)
				return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private %s::__destruct() from global scope", zobj.GetClass().GetName()))
			}
		} else {
			if callerClass == nil || (!callerClass.InstanceOf(zobj.GetClass()) && !zobj.GetClass().InstanceOf(callerClass)) {
				scope := "global scope"
				if callerClass != nil {
					scope = fmt.Sprintf("scope %s", callerClass.GetName())
				}
				ctx.Global().UnregisterDestructor(zobj)
				return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__destruct() from %s", zobj.GetClass().GetName(), scope))
			}
		}
	}
	ctx.Global().UnregisterDestructor(zobj)
	_, err := ctx.CallZVal(ctx, m.Method, nil, zobj)
	return err
}

func compileUnset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	un := &runnableUnset{l: i.Loc()}
	un.args, err = compileFuncPassedArgs(c)
	return un, err
}

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
			// For array access on ArrayAccess objects, skip reading the value
			// (unset should only call offsetUnset, not offsetGet)
			if _, isArrayAccess := v.(*runArrayAccess); !isArrayAccess {
				// Before unsetting, check if the value is an object with __destruct
				zv, runErr := v.Run(ctx)
				if runErr != nil {
					return nil, runErr
				}
				if zv != nil && zv.GetType() == phpv.ZtObject {
					obj := zv.Value()
					if zobj, ok := obj.(phpv.ZObject); ok {
						if m, ok := zobj.GetClass().GetMethod("__destruct"); ok {
							// Check destructor visibility
							if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
								callerClass := ctx.Class()
								if m.Modifiers.IsPrivate() {
									if callerClass == nil || callerClass.GetName() != zobj.GetClass().GetName() {
										// Unregister from shutdown destructors before throwing to prevent duplicate warning from Close()
										ctx.Global().UnregisterDestructor(zobj)
										return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private %s::__destruct() from global scope", zobj.GetClass().GetName()))
									}
								} else {
									if callerClass == nil || (!callerClass.InstanceOf(zobj.GetClass()) && !zobj.GetClass().InstanceOf(callerClass)) {
										scope := "global scope"
										if callerClass != nil {
											scope = fmt.Sprintf("scope %s", callerClass.GetName())
										}
										// Unregister from shutdown destructors before throwing to prevent duplicate warning from Close()
										ctx.Global().UnregisterDestructor(zobj)
										return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__destruct() from %s", zobj.GetClass().GetName(), scope))
									}
								}
							}
							ctx.Global().UnregisterDestructor(zobj)
							ctx.CallZVal(ctx, m.Method, nil, zobj)
						}
					}
				}
			}
			x.WriteValue(ctx, nil)
		} else {
			return nil, ctx.Errorf("unable to unset value")
		}
	}
	return nil, nil
}

func compileUnset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	un := &runnableUnset{l: i.Loc()}
	un.args, err = compileFuncPassedArgs(c)
	return un, err
}

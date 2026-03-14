package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableClone struct {
	arg phpv.Runnable
	l   *phpv.Loc
}

func (r *runnableClone) Dump(w io.Writer) error {
	_, err := w.Write([]byte("clone "))
	if err != nil {
		return err
	}
	return r.arg.Dump(w)
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

	obj, err = obj.Clone(ctx)
	if err != nil {
		return nil, err
	}

	return obj.ZVal(), nil
}

func compileClone(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	cl := &runnableClone{l: i.Loc()}
	cl.arg, err = compileExpr(nil, c)
	return cl, err
}

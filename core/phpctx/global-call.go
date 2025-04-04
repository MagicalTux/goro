package phpctx

import (
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// perform call in new context
func (c *Global) Call(ctx phpv.Context, f phpv.Callable, args []phpv.Runnable, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	var zArgs []*phpv.ZVal
	for _, arg := range args {
		val, err := arg.Run(ctx)
		if err != nil {
			return nil, err
		}
		zArgs = append(zArgs, val)
	}
	return c.CallZVal(ctx, f, zArgs, optionalThis...)
}

func (c *Global) CallZVal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       phpv.NewHashTable(),
		c:       f,
		loc:     ctx.Loc(),
	}

	var this phpv.ZObject
	if len(optionalThis) > 0 {
		this = optionalThis[0]
	}

	if this == nil {
		if obj, ok := f.(*phpv.BoundedCallable); ok {
			this = obj.This
			args = append(obj.Args, args...)

		}
	}

	if m, ok := f.(*phpv.MethodCallable); ok {
		callCtx.class = m.Class
		if m.Static {
			callCtx.methodType = "::"
		} else {
			callCtx.methodType = "->"
		}
	} else if this != nil {
		callCtx.class = this.GetClass()
		callCtx.methodType = "->"
	}

	callCtx.this = this

	// collect args
	// use func_args to check if any arg is a ref and needs to be passed as such
	if c, ok := f.(phpv.FuncGetArgs); ok {
		// This function is defined inside a PHP script
		func_args := c.GetArgs()
		callCtx.Args = args
		for i := range args {
			// Since this function was parsed, the parameter info is available
			if i < len(func_args) && func_args[i].Ref {
				callCtx.Args[i] = callCtx.Args[i].Ref()

				// Handle case foo($bar) where $bar is undefined
				// and foo takes a reference
				ctx.OffsetSet(ctx, callCtx.Args[i].GetName(), callCtx.Args[i])
			} else {
				argName := args[i].GetName()
				if argName != "" {
					if ok, _ := ctx.OffsetExists(ctx, argName); !ok {
						if err := ctx.Notice("Undefined variable: %s",
							argName, logopt.NoFuncName(true)); err != nil {
							return nil, err
						}
					}
				}
				callCtx.Args[i] = callCtx.Args[i].Dup()
			}
		}
	} else {
		// This function is defined inside go,
		// let the Go function decide whether to Dup() or Ref() the args
		// since the parameter info (such as isReference) is not available.
		// To mark a parameter as reference instead, do:
		// var arg3 core.Ref[phpv.ZInt]
		// core.Expand(ctx, args, &arg1, &arg3)
		callCtx.Args = args
	}

	return phperr.CatchReturn(f.Call(callCtx, callCtx.Args))
}

func (c *Global) Parent(n int) phpv.Context {
	return nil
}

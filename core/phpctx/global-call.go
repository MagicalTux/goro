package phpctx

import (
	"fmt"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
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
	c.callDepth++
	if c.callDepth > 512 {
		c.callDepth--
		return nil, ctx.Errorf("Maximum function nesting level of '512' reached, aborting!")
	}
	callCtx := GetFuncContext()
	callCtx.Context = ctx
	callCtx.c = f
	callCtx.loc = ctx.Loc()
	defer func() {
		callCtx.Release()
		c.callDepth--
	}()

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
						if err := ctx.Warn("Undefined variable $%s",
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

	// Check type hints
	if fga, ok := f.(phpv.FuncGetArgs); ok {
		funcArgs := fga.GetArgs()
		for i, fa := range funcArgs {
			if fa.Hint == nil {
				continue
			}
			if i >= len(callCtx.Args) {
				break
			}
			val := callCtx.Args[i]
			if val.IsNull() && !fa.Required {
				continue // allow null for optional params
			}
			if !fa.Hint.Check(callCtx, val) {
				// Get the actual type name for the error
				actualType := phpTypeName(val)
				funcName := callCtx.GetFuncName()
				msg := fmt.Sprintf("%s(): Argument #%d ($%s) must be of type %s, %s given", funcName, i+1, fa.VarName, fa.Hint.String(), actualType)
				// Add call location and definition location
				var defLoc *phpv.Loc
				if dl, ok := f.(interface{ Loc() *phpv.Loc }); ok {
					defLoc = dl.Loc()
				}
				if callLoc := ctx.Loc(); callLoc != nil {
					msg += fmt.Sprintf(", called in %s on line %d", callLoc.Filename, callLoc.Line)
				}
				return nil, phpobj.ThrowErrorAt(callCtx, phpobj.TypeError, msg, defLoc)
			}
		}
	}

	return phperr.CatchReturn(f.Call(callCtx, callCtx.Args))
}

func phpTypeName(val *phpv.ZVal) string {
	if val.GetType() == phpv.ZtObject {
		if obj, ok := val.Value().(phpv.ZObject); ok {
			return string(obj.GetClass().GetName())
		}
	}
	return val.GetType().TypeName()
}

func (c *Global) Parent(n int) phpv.Context {
	return nil
}

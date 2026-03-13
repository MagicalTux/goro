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
	// Pre-check: for reference parameters, validate expression types before evaluation
	var funcArgs []*phpv.FuncArg
	if fga, ok := f.(phpv.FuncGetArgs); ok {
		funcArgs = fga.GetArgs()
	}
	// Also check Go-implemented functions with ExtFunctionArg metadata
	var extArgs []*ExtFunctionArg
	if ef, ok := f.(*ExtFunction); ok {
		extArgs = ef.Args
	}

	// Save call site location (arg evaluation may change global location)
	callLoc := ctx.Loc()

	var zArgs []*phpv.ZVal
	for i, arg := range args {
		isRefParam := false
		if funcArgs != nil && i < len(funcArgs) && funcArgs[i].Ref {
			isRefParam = true
		} else if extArgs != nil && i < len(extArgs) && extArgs[i].Ref {
			isRefParam = true
		}

		// Emit "Undefined variable" warning for by-value params that are
		// simple variables. This check must happen BEFORE arg.Run() since
		// Run() suppresses warnings for function call contexts.
		if !isRefParam {
			if uc, ok := arg.(phpv.UndefinedChecker); ok {
				if uc.IsUnDefined(ctx) {
					ctx.Warn("Undefined variable $%s",
						uc.VarName(), logopt.NoFuncName(true))
				}
			}
		}

		// For by-ref params, put array access expressions into write context
		// to suppress "Trying to access array offset on null" warnings and
		// enable auto-vivification (e.g., foo($undef[0]) should create $undef).
		if isRefParam {
			if wcs, ok := arg.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(true)
			}
		}

		val, err := arg.Run(ctx)
		if err != nil {
			// Reset write context before returning error
			if isRefParam {
				if wcs, ok := arg.(phpv.WriteContextSetter); ok {
					wcs.SetWriteContext(false)
				}
			}
			return nil, err
		}

		if isRefParam {
			writable, isWritable := arg.(phpv.Writable)
			if !isWritable && !val.IsRef() {
				// Non-variable, non-reference result passed to a by-ref parameter
				if _, isFuncCall := arg.(phpv.FuncCallExpression); isFuncCall {
					// Function/method call -> Notice, pass by value
					// Restore call site location for correct notice line
					ctx.Tick(ctx, callLoc)
					ctx.Notice("Only variables should be passed by reference",
						logopt.NoFuncName(true))
					val = val.Dup()
					val.Name = nil // clear Name so CallZVal skips ref processing
				} else {
					// Literal, assignment, or other non-variable expression -> Error
					funcName := "unknown"
					if namer, ok := f.(interface{ Name() string }); ok {
						funcName = namer.Name()
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("%s(): Argument #%d ($%s) could not be passed by reference",
							funcName, i+1, funcArgs[i].VarName))
				}
			} else if isWritable && !val.IsRef() {
				// Writable source (array element, object property) passed by ref:
				// create the reference and write it back to the source so the
				// source element also becomes a reference.
				ref := val.Ref()
				writable.WriteValue(ctx, ref)
				val = ref
			}
			// Reset write context after all by-ref handling (including WriteValue)
			if wcs, ok := arg.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(false)
			}
		}

		// For non-reference parameters in PHP-defined functions, Dup the value
		// immediately so that later argument evaluations (e.g., $x = 1) don't
		// retroactively change values already captured for earlier arguments.
		// For Go ext functions, Expand() handles Dup internally, and premature
		// Dup would break array internal pointer sharing.
		if !isRefParam && funcArgs != nil {
			val = val.Dup()
		}

		zArgs = append(zArgs, val)
	}
	return c.CallZVal(ctx, f, zArgs, optionalThis...)
}

// CallZValInternal is like CallZVal but marks the call as internal (e.g., from output buffer callbacks).
// This causes the stack trace entry to show "[internal function]" instead of the filename.
func (c *Global) CallZValInternal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	return c.callZValImpl(ctx, f, args, true, optionalThis...)
}

func (c *Global) CallZVal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	return c.callZValImpl(ctx, f, args, false, optionalThis...)
}

func (c *Global) callZValImpl(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, isInternal bool, optionalThis ...phpv.ZObject) (*phpv.ZVal, error) {
	c.callDepth++
	if c.callDepth > 512 {
		c.callDepth--
		return nil, ctx.Errorf("Maximum function nesting level of '512' reached, aborting!")
	}
	callCtx := GetFuncContext()
	callCtx.Context = ctx
	callCtx.c = f
	callCtx.loc = ctx.Loc()
	callCtx.isInternal = isInternal
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
				// Check if the argument can be passed by reference
				argName := callCtx.Args[i].GetName()
				if argName == "" {
					// No variable name - either handled in Call() with a Notice
					// (function call result), or a direct CallZVal with a non-variable.
					// In either case, pass by value instead of by reference.
					if !callCtx.Args[i].IsRef() {
						callCtx.Args[i] = callCtx.Args[i].Dup()
						continue
					}
				}
				callCtx.Args[i] = callCtx.Args[i].Ref()

				// Handle case foo($bar) where $bar is undefined
				// and foo takes a reference
				if argName != "" {
					ctx.OffsetSet(ctx, callCtx.Args[i].GetName(), callCtx.Args[i])
				}
			} else {
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

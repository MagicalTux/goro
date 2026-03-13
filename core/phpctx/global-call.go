package phpctx

import (
	"fmt"
	"io"

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

	// PHP 8.0 Named Arguments: reorder args to match parameter positions
	if funcArgs != nil {
		args = reorderNamedArgs(ctx, funcArgs, args)
	}

	// Expand spread arguments (...$arr) into individual args before evaluation
	args = expandSpreadArgs(ctx, args)

	// Save call site location (arg evaluation may change global location)
	callLoc := ctx.Loc()

	var zArgs []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal // refs to unwrap after call returns
	for i, arg := range args {
		// Unwrap named arguments (already reordered to correct position)
		if na, ok := arg.(phpv.NamedArgument); ok {
			arg = na.Inner()
		}

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
				// Record for cleanup: when the function returns, the ref
				// wrapper should be removed (PHP does this via refcounting;
				// when refcount drops to 1, is_ref is cleared).
				byRefCleanups = append(byRefCleanups, ref)
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
	result, err := c.CallZVal(ctx, f, zArgs, optionalThis...)

	// After the call returns, unwrap by-ref parameters that were created
	// during this call. In PHP, when refcount drops to 1 (the function
	// parameter goes out of scope), the is_ref flag is cleared. Since goro
	// doesn't have refcounting, we do it explicitly here.
	for _, ref := range byRefCleanups {
		ref.UnRef()
	}

	return result, err
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

		// Handle variadic parameter: pack remaining args into an array
		variadicIdx := -1
		for i, fa := range func_args {
			if fa.Variadic {
				variadicIdx = i
				break
			}
		}

		if variadicIdx >= 0 && len(args) >= variadicIdx {
			// Pack arguments from variadicIdx onward into a ZArray
			varArray := phpv.NewZArray()
			for _, a := range args[variadicIdx:] {
				varArray.OffsetSet(nil, nil, a.Dup())
			}
			// Replace args: keep args before variadic, then add the packed array
			newArgs := make([]*phpv.ZVal, variadicIdx+1)
			copy(newArgs, args[:variadicIdx])
			newArgs[variadicIdx] = varArray.ZVal()
			args = newArgs
		}

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
			// For variadic params, the arg is already packed as an array.
			// Type check each element of the array, not the array itself.
			if fa.Variadic {
				arr := callCtx.Args[i].AsArray(callCtx)
				if arr != nil {
					elemIdx := 0
					for _, elem := range arr.Iterate(callCtx) {
						if !fa.Hint.Check(callCtx, elem) {
							actualType := phpTypeName(elem)
							funcName := callCtx.GetFuncName()
							msg := fmt.Sprintf("%s(): Argument #%d ($%s) must be of type %s, %s given", funcName, i+elemIdx+1, fa.VarName, fa.Hint.String(), actualType)
							var defLoc *phpv.Loc
							if dl, ok := f.(interface{ Loc() *phpv.Loc }); ok {
								defLoc = dl.Loc()
							}
							if !callCtx.isInternal {
								if callLoc := ctx.Loc(); callLoc != nil {
									msg += fmt.Sprintf(", called in %s on line %d", callLoc.Filename, callLoc.Line)
								}
							}
							return nil, phpobj.ThrowErrorAt(callCtx, phpobj.TypeError, msg, defLoc)
						}
						elemIdx++
					}
				}
				continue
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
				if !callCtx.isInternal {
					if callLoc := ctx.Loc(); callLoc != nil {
						msg += fmt.Sprintf(", called in %s on line %d", callLoc.Filename, callLoc.Line)
					}
				}
				return nil, phpobj.ThrowErrorAt(callCtx, phpobj.TypeError, msg, defLoc)
			}
		}
	}

	return phperr.CatchReturn(f.Call(callCtx, callCtx.Args))
}

// reorderNamedArgs reorders function call arguments based on PHP 8.0 named argument syntax.
// Positional arguments must come before named arguments.
// Named arguments are placed at their corresponding parameter position.
func reorderNamedArgs(ctx phpv.Context, funcArgs []*phpv.FuncArg, args []phpv.Runnable) []phpv.Runnable {
	// Check if any args are named
	hasNamed := false
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return args
	}

	// Build result array sized to max(len(funcArgs), len(args))
	size := len(funcArgs)
	if len(args) > size {
		size = len(args)
	}
	result := make([]phpv.Runnable, size)

	// Place positional arguments first
	positionalEnd := 0
	for i, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			break
		}
		result[i] = arg
		positionalEnd = i + 1
	}

	// Place named arguments at their parameter positions
	for _, arg := range args[positionalEnd:] {
		na, ok := arg.(phpv.NamedArgument)
		if !ok {
			// Positional after named - this is a PHP error
			// For now, just skip (PHP would throw an error)
			continue
		}
		name := na.ArgName()
		found := false
		for j, fa := range funcArgs {
			if fa.VarName == name {
				result[j] = arg
				found = true
				break
			}
		}
		if !found {
			// Unknown named parameter - append at end, CallZVal will handle the error
			result = append(result, arg)
		}
	}

	// Trim trailing nil entries
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
	}

	return result
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

// expandSpreadArgs expands any SpreadArgument entries by evaluating the
// expression and creating a runZVal wrapper for each element of the result array.
// Non-spread arguments are passed through unchanged.
func expandSpreadArgs(ctx phpv.Context, args []phpv.Runnable) []phpv.Runnable {
	// Quick check: any spread args?
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(phpv.SpreadArgument); ok {
			hasSpread = true
			break
		}
	}
	if !hasSpread {
		return args
	}

	result := make([]phpv.Runnable, 0, len(args))
	for _, arg := range args {
		sa, ok := arg.(phpv.SpreadArgument)
		if !ok {
			result = append(result, arg)
			continue
		}
		// Evaluate the spread expression
		val, err := sa.Inner().Run(ctx)
		if err != nil {
			// Can't return error from here, wrap as a runnable that will error
			result = append(result, arg)
			continue
		}
		if val.GetType() != phpv.ZtArray {
			// Non-array spread: just pass the value
			result = append(result, &spreadZVal{val})
			continue
		}
		arr := val.AsArray(ctx)
		for _, v := range arr.Iterate(ctx) {
			result = append(result, &spreadZVal{v.Dup()})
		}
	}
	return result
}

// spreadZVal is a Runnable wrapper for pre-evaluated spread argument values.
type spreadZVal struct {
	v *phpv.ZVal
}

func (s *spreadZVal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return s.v, nil
}

func (s *spreadZVal) Dump(w io.Writer) error {
	_, err := w.Write([]byte("spread_val"))
	return err
}

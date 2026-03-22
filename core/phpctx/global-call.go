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

	// Expand spread arguments (...$arr) into individual args before evaluation.
	// This must happen before named arg reordering because spread of string-keyed
	// arrays produces named args that need to be reordered.
	args, err := expandSpreadArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	// PHP 8.0 Named Arguments: reorder args to match parameter positions
	if funcArgs != nil {
		funcName := ""
		if fn, ok := f.(interface{ GetFuncName() phpv.ZString }); ok {
			funcName = string(fn.GetFuncName())
		}
		var namedErr error
		args, namedErr = reorderNamedArgs(ctx, funcArgs, args, funcName)
		if namedErr != nil {
			return nil, namedErr
		}
	}

	// Save call site location (arg evaluation may change global location)
	callLoc := ctx.Loc()

	var zArgs []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal
	for i, arg := range args {
		// nil entries come from reorderNamedArgs when a named argument
		// skips a position (the callee will use its default value).
		if arg == nil {
			zArgs = append(zArgs, nil)
			continue
		}
		// Unwrap named arguments (already reordered to correct position)
		if na, ok := arg.(phpv.NamedArgument); ok {
			arg = na.Inner()
		}

		isRefParam := false
		isPreferRef := false
		if funcArgs != nil && i < len(funcArgs) && funcArgs[i].Ref {
			isRefParam = true
			isPreferRef = funcArgs[i].PreferRef
		} else if funcArgs != nil && len(funcArgs) > 0 {
			// Check if the last parameter is a variadic by-ref param;
			// if so, all args from that index onward are by-ref.
			last := funcArgs[len(funcArgs)-1]
			if last.Variadic && last.Ref && i >= len(funcArgs)-1 {
				isRefParam = true
				isPreferRef = last.PreferRef
			}
		}
		if !isRefParam && extArgs != nil && i < len(extArgs) && extArgs[i].Ref {
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
			_, isWritable := arg.(phpv.Writable)
			if !isWritable && !val.IsRef() {
				// Non-variable, non-reference result passed to a by-ref parameter
				if isPreferRef {
					// ZEND_SEND_PREFER_REF: silently accept non-ref values
					val = val.Dup()
					val.Name = nil
				} else if _, isFuncCall := arg.(phpv.FuncCallExpression); isFuncCall {
					// Function/method call -> Notice, pass by value
					// Restore call site location for correct notice line
					ctx.Tick(ctx, callLoc)
					ctx.Notice("Only variables should be passed by reference",
						logopt.NoFuncName(true))
					val = val.Dup()
					// Mark as already warned about by-ref issue. Use a sentinel
					// name so CallZVal knows not to emit a second warning.
					alreadyWarned := phpv.ZString("\x00ref_warned")
					val.Name = &alreadyWarned
				} else {
					// Literal, assignment, or other non-variable expression -> Error
					funcName := phpv.CallableDisplayName(f)
					if funcName == "" {
						funcName = "unknown"
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("%s(): Argument #%d ($%s) could not be passed by reference",
							funcName, i+1, funcArgs[i].VarName))
				}
			} else if isWritable && !val.IsRef() {
				// For compound writable expressions (array elements, object
				// properties), we need to ensure the element exists
				// (auto-vivification) and set up a reference that gets cleaned
				// up after the call. For simple variables, the existing ref
				// mechanism in callZValImpl handles everything.
				if _, isCompound := arg.(phpv.CompoundWritable); isCompound {
					// Check if creating a reference would violate readonly constraints
					if rc, ok := arg.(phpv.ReadonlyRefChecker); ok {
						if err := rc.CheckReadonlyRef(ctx); err != nil {
							if wcs, ok := arg.(phpv.WriteContextSetter); ok {
								wcs.SetWriteContext(false)
							}
							return nil, err
						}
					}
					writable := arg.(phpv.Writable)
					writable.WriteValue(ctx, val.Dup())
					// Re-read to get the actual hash table entry ZVal.
					val, _ = arg.Run(ctx)
					// Make the hash table entry into a reference in-place.
					val.MakeRef()
					byRefCleanups = append(byRefCleanups, val)
				} else if sv, isSpread := arg.(*spreadZVal); isSpread && !sv.fromLiteral {
					// Spread from a variable: make the hash table entry a reference
					// so modifications propagate back to the source array.
					val.MakeRef()
					byRefCleanups = append(byRefCleanups, val)
				}
			}
			// Reset write context after all by-ref handling
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

	// After the call returns, unwrap by-ref hash table entries that were
	// made into references during this call — but only if no other location
	// still references the same inner ZVal (e.g. the callee stored it via
	// $this->prop = &$param). This approximates PHP's refcount-based un-ref.
	for _, ref := range byRefCleanups {
		ref.UnRefIfAlone()
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

func (c *Global) callZValImpl(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, isInternal bool, optionalThis ...phpv.ZObject) (callResult *phpv.ZVal, callErr error) {
	c.callDepth++
	if c.callDepth > 4096 {
		c.callDepth--
		return nil, ctx.Errorf("Maximum function nesting level of '4096' reached, aborting!")
	}
	callCtx := GetFuncContext()
	callCtx.Context = ctx
	callCtx.c = f
	callCtx.loc = ctx.Loc()
	callCtx.isInternal = isInternal
	// Release() may trigger destructor errors during scope cleanup.
	// Named return values let the defer closure chain those errors
	// with any pending call error before the function returns.
	defer func() {
		releaseErr := callCtx.Release()
		c.callDepth--
		if releaseErr != nil && callErr != nil {
			// Both the call and a destructor during scope cleanup threw.
			// PHP chains the pending exception as the "previous" of the
			// destructor exception, so the destructor error propagates.
			callErr = chainPhpThrow(releaseErr, callErr)
		} else if releaseErr != nil {
			callErr = releaseErr
		}
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
		if m.CalledClass != nil {
			callCtx.calledClass = m.CalledClass
		} else {
			callCtx.calledClass = m.Class
		}
		if m.Static {
			callCtx.methodType = "::"
			// Static methods don't have $this, even when called on an instance
			this = nil
		} else {
			callCtx.methodType = "->"
		}
	} else if zc, ok := f.(phpv.ZClosure); ok {
		// For closures, the scope (class) determines visibility, not $this's class.
		if cls := zc.GetClass(); cls != nil {
			callCtx.class = cls
		} else if this != nil {
			callCtx.class = this.GetClass()
		}
		if this != nil {
			callCtx.methodType = "->"
		} else if callCtx.class != nil {
			// Static closures or closures without $this that have a class scope
			// should display ClassName:: in stack traces and error messages.
			callCtx.methodType = "::"
		}
	} else if this != nil {
		callCtx.class = this.GetClass()
		callCtx.methodType = "->"
	}

	// For closures with a class scope but no $this, set the class on the call context
	// so that self:: and static:: resolve correctly inside the closure body.
	if callCtx.class == nil {
		if zc, ok := f.(phpv.ZClosure); ok {
			if cls := zc.GetClass(); cls != nil {
				callCtx.class = cls
			}
		}
	}
	// For callables with a class scope (e.g., generator body callables),
	// set the class context so get_class()/self::/static:: work.
	if callCtx.class == nil {
		if cg, ok := f.(interface{ GetClass() phpv.ZClass }); ok {
			if cls := cg.GetClass(); cls != nil {
				callCtx.class = cls
				if this != nil {
					callCtx.methodType = "->"
				} else {
					callCtx.methodType = "::"
				}
			}
		}
	}
	// Set called class for late static binding (static::class)
	if callCtx.calledClass == nil {
		if cc, ok := f.(interface{ GetCalledClass() phpv.ZClass }); ok {
			if called := cc.GetCalledClass(); called != nil {
				callCtx.calledClass = called
			}
		}
	}

	callCtx.this = this

	// collect args
	// use func_args to check if any arg is a ref and needs to be passed as such
	var func_args []*phpv.FuncArg
	if c, ok := f.(phpv.FuncGetArgs); ok {
		func_args = c.GetArgs()
	}
	if func_args != nil {

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
			isRefVariadic := func_args[variadicIdx].Ref
			for _, a := range args[variadicIdx:] {
				if a == nil {
					continue // skip nil gaps from named arg reordering
				}
				if isRefVariadic {
					// For by-ref variadic, make each element a reference so that
					// modifications inside the function propagate back to the
					// source array (when spread from a variable).
					a.MakeRef()
					varArray.OffsetSet(nil, nil, a.Ref())
				} else {
					varArray.OffsetSet(nil, nil, a.Dup())
				}
			}
			// Replace args: keep args before variadic, then add the packed array
			newArgs := make([]*phpv.ZVal, variadicIdx+1)
			copy(newArgs, args[:variadicIdx])
			newArgs[variadicIdx] = varArray.ZVal()
			args = newArgs
		}

		callCtx.Args = args
		for i := range args {
			// Skip variadic parameter - already handled during packing above
			if variadicIdx >= 0 && i == variadicIdx {
				continue
			}
			// nil entries come from named arg reordering when a position was skipped;
			// the callee will fill in the default value, so leave them nil.
			if callCtx.Args[i] == nil {
				continue
			}
			// Since this function was parsed, the parameter info is available
			if i < len(func_args) && func_args[i].Ref {
				argName := callCtx.Args[i].GetName()
				if argName == "" || argName == "\x00ref_warned" {
					// No variable name - either handled in Call() with a Notice
					// (function call result), or a direct CallZVal with a non-variable.
					// In either case, pass by value instead of by reference.
					if argName == "\x00ref_warned" {
						// Already warned by Call() with "Only variables should be
						// passed by reference" Notice - skip the second warning.
						callCtx.Args[i] = callCtx.Args[i].Dup()
						continue
					}
					if !callCtx.Args[i].IsRef() {
						// PreferRef params (ZEND_SEND_PREFER_REF) silently accept
						// non-ref values without warning (e.g. array_multisort).
						if func_args[i].PreferRef {
							callCtx.Args[i] = callCtx.Args[i].Dup()
							continue
						}
						// Emit "must be passed by reference, value given" warning
						// (e.g. when call_user_func_array passes non-ref to a ref param)
						funcName := callCtx.GetFuncName()
						ctx.Warn("%s(): Argument #%d ($%s) must be passed by reference, value given",
							funcName, i+1, func_args[i].VarName, logopt.NoFuncName(true))
						callCtx.Args[i] = callCtx.Args[i].Dup()
						continue
					}
				}
				// Convert the argument to a reference in-place so that the
				// original location (hash table entry, object property, etc.)
				// is also marked as a reference, matching PHP behavior.
				callCtx.Args[i].MakeRef()
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
	// strict_types comes from the CALLING file (ctx), not the callee
	isStrict := c.StrictTypes
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
						var typeOk bool
						if isStrict {
							typeOk = fa.Hint.CheckStrict(callCtx, elem)
						} else {
							typeOk = fa.Hint.Check(callCtx, elem)
						}
						if !typeOk {
							actualType := phpTypeName(elem)
							if isStrict {
								actualType = phpTypeNameDetailed(elem)
							}
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
			var typeOk bool
			if isStrict {
				typeOk = fa.Hint.CheckStrict(callCtx, val)
			} else {
				typeOk = fa.Hint.Check(callCtx, val)
			}
			if !typeOk {
				// Get the actual type name for the error
				actualType := phpTypeName(val)
				if isStrict {
					actualType = phpTypeNameDetailed(val)
				}
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

	callResult, callErr = phperr.CatchReturn(f.Call(callCtx, callCtx.Args))
	if hasNoDiscardAttr(f) {
		// Wrap with the runtime object (this) so NoDiscard warnings
		// report the correct class name (e.g. for trait methods).
		if this != nil {
			if _, alreadyBound := f.(*phpv.BoundedCallable); !alreadyBound {
				c.lastCallable = phpv.Bind(f, this)
			} else {
				c.lastCallable = f
			}
		} else {
			c.lastCallable = f
		}
	}

	// For functions that do NOT return by reference, separate array values
	// so that writing to the returned array doesn't modify the original
	// (PHP copy-on-write semantics). Objects are excluded since they always
	// have reference semantics.
	if callErr == nil && callResult != nil && callResult.GetType() == phpv.ZtArray {
		returnsByRef := false
		if rr, ok := f.(interface{ ReturnsByRef() bool }); ok {
			returnsByRef = rr.ReturnsByRef()
		}
		if !returnsByRef {
			// Dup creates a COW clone; force a full separation so that
			// in-place modifications (e.g. doInc on $foo->f1()[0]++)
			// don't affect the original array's elements via shared ZVal pointers.
			callResult = callResult.Dup()
			if za, ok := callResult.Value().(*phpv.ZArray); ok {
				za.SeparateCow()
			}
		}
	}

	return callResult, callErr
}

// hasNoDiscardAttr checks if a callable has NoDiscard attributes.
func hasNoDiscardAttr(c phpv.Callable) bool {
	switch v := c.(type) {
	case *phpv.BoundedCallable:
		return hasNoDiscardAttr(v.Callable)
	case *phpv.MethodCallable:
		return hasNoDiscardAttr(v.Callable)
	}
	if ag, ok := c.(phpv.AttributeGetter); ok {
		for _, attr := range ag.GetAttributes() {
			if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
				return true
			}
		}
	}
	return false
}

// chainPhpThrow chains two PHP exceptions: when a destructor throws
// (destructorErr) while another exception is already pending (pendingErr),
// PHP sets the pending exception as the "previous" of the destructor
// exception. The destructor exception then propagates.
func chainPhpThrow(destructorErr, pendingErr error) error {
	dThrow, dok := destructorErr.(*phperr.PhpThrow)
	pThrow, pok := pendingErr.(*phperr.PhpThrow)
	if dok && pok && dThrow.Obj != nil && pThrow.Obj != nil {
		// Set pendingErr's exception object as "previous" on destructorErr's
		// exception object, so the output chains them with "Next".
		dThrow.Obj.HashTable().SetString("previous", pThrow.Obj.ZVal())
	}
	return destructorErr
}

// reorderNamedArgs reorders function call arguments based on PHP 8.0 named argument syntax.
// Positional arguments must come before named arguments.
// Named arguments are placed at their corresponding parameter position.
// Returns an error for unknown named parameters or duplicate parameter names.
func reorderNamedArgs(ctx phpv.Context, funcArgs []*phpv.FuncArg, args []phpv.Runnable, funcName string) ([]phpv.Runnable, error) {
	// Check if any args are named
	hasNamed := false
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return args, nil
	}

	// Build result array sized to max(len(funcArgs), len(args))
	size := len(funcArgs)
	if len(args) > size {
		size = len(args)
	}
	result := make([]phpv.Runnable, size)

	// Track which positions are filled (for duplicate detection)
	filled := make([]bool, size)

	// Place positional arguments first
	positionalEnd := 0
	for i, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			break
		}
		result[i] = arg
		filled[i] = true
		positionalEnd = i + 1
	}

	// Check if the last funcArg is variadic
	hasVariadic := false
	if len(funcArgs) > 0 {
		hasVariadic = funcArgs[len(funcArgs)-1].Variadic
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
				if filled[j] {
					// Duplicate named parameter: named param overwrites positional or another named
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Named parameter $%s overwrites previous argument", name))
				}
				result[j] = arg
				filled[j] = true
				found = true
				break
			}
		}
		if !found {
			if hasVariadic {
				// Named args for variadic parameters are collected into the variadic array
				result = append(result, arg)
			} else {
				// Unknown named parameter
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Unknown named parameter $%s", name))
			}
		}
	}

	// Trim trailing nil entries
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
	}

	return result, nil
}

func phpTypeName(val *phpv.ZVal) string {
	if val.GetType() == phpv.ZtObject {
		if obj, ok := val.Value().(phpv.ZObject); ok {
			return string(obj.GetClass().GetName())
		}
	}
	return val.GetType().TypeName()
}

// phpTypeNameDetailed returns the PHP type name with "true"/"false" for booleans
// (used in strict mode error messages).
func phpTypeNameDetailed(val *phpv.ZVal) string {
	return phpv.ZValTypeNameDetailed(val)
}

func (c *Global) Parent(n int) phpv.Context {
	return nil
}

// expandSpreadArgs expands any SpreadArgument entries by evaluating the
// expression and creating a runZVal wrapper for each element of the result array.
// Non-spread arguments are passed through unchanged.
// Returns an error (TypeError) if a non-array/Traversable value is spread.
func expandSpreadArgs(ctx phpv.Context, args []phpv.Runnable) ([]phpv.Runnable, error) {
	// Quick check: any spread args?
	// NamedArgument also satisfies SpreadArgument (both have Inner()),
	// so we must exclude NamedArgument from spread detection.
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			continue // named args are not spread args
		}
		if _, ok := arg.(phpv.SpreadArgument); ok {
			hasSpread = true
			break
		}
	}
	if !hasSpread {
		return args, nil
	}

	result := make([]phpv.Runnable, 0, len(args))
	for _, arg := range args {
		// Skip named arguments - they satisfy SpreadArgument interface
		// but should not be treated as spread/unpack operations.
		if _, ok := arg.(phpv.NamedArgument); ok {
			result = append(result, arg)
			continue
		}
		sa, ok := arg.(phpv.SpreadArgument)
		if !ok {
			result = append(result, arg)
			continue
		}
		// Evaluate the spread expression
		inner := sa.Inner()
		val, err := inner.Run(ctx)
		if err != nil {
			return nil, err
		}
		// Detect if spread source is a variable (can be passed by reference)
		_, isWritable := inner.(phpv.Writable)
		if val.GetType() == phpv.ZtArray {
			arr := val.AsArray(ctx)
			if isWritable {
				// From a variable: pass the actual hash table entries (for by-ref)
				// so that modifications inside the function propagate back.
				// Force COW separation first so we don't modify shared data.
				arr.SeparateCow()
				it := arr.NewIterator()
				seenStringKey := false
				for it.Valid(ctx) {
					k, _ := it.Key(ctx)
					// Use CurrentRef to get the actual ZVal without duplication
					v, _ := it.(interface {
						CurrentRef(phpv.Context) (*phpv.ZVal, error)
					}).CurrentRef(ctx)
					if v != nil {
						entry := phpv.Runnable(&spreadZVal{v: v, fromLiteral: false})
						// String keys become named arguments
						if k != nil && k.GetType() == phpv.ZtString {
							entry = &spreadNamedArg{name: phpv.ZString(k.String()), inner: entry}
							seenStringKey = true
						} else if seenStringKey {
							// Positional after named during unpacking
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								"Cannot use positional argument after named argument during unpacking")
						}
						result = append(result, entry)
					}
					it.Next(ctx)
				}
			} else {
				// From a literal: dup the values
				seenStringKey := false
				for k, v := range arr.Iterate(ctx) {
					entry := phpv.Runnable(&spreadZVal{v: v.Dup(), fromLiteral: true})
					// String keys become named arguments
					if k != nil && k.GetType() == phpv.ZtString {
						entry = &spreadNamedArg{name: phpv.ZString(k.String()), inner: entry}
						seenStringKey = true
					} else if seenStringKey {
						// Positional after named during unpacking
						return nil, phpobj.ThrowError(ctx, phpobj.Error,
							"Cannot use positional argument after named argument during unpacking")
					}
					result = append(result, entry)
				}
			}
			continue
		}
		// Check for Traversable objects (Generator, Iterator, IteratorAggregate)
		if val.GetType() == phpv.ZtObject {
			if obj, ok := val.Value().(*phpobj.ZObject); ok {
				if obj.GetClass().Implements(phpobj.IteratorAggregate) {
					iterResult, err := obj.CallMethod(ctx, "getIterator")
					if err != nil {
						return nil, err
					}
					if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
						if iterObj, ok := iterResult.Value().(*phpobj.ZObject); ok && iterObj.GetClass().Implements(phpobj.Iterator) {
							obj = iterObj
						}
					}
				}
				if obj.GetClass().Implements(phpobj.Iterator) {
					if _, err := obj.CallMethod(ctx, "rewind"); err != nil {
						return nil, err
					}
					seenStringKey := false
					for {
						v, err := obj.CallMethod(ctx, "valid")
						if err != nil {
							return nil, err
						}
						if !v.AsBool(ctx) {
							break
						}
						key, kerr := obj.CallMethod(ctx, "key")
						if kerr != nil {
							return nil, kerr
						}
						// Validate key type: must be int or string
						if key.GetType() != phpv.ZtInt && key.GetType() != phpv.ZtString {
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								"Keys must be of type int|string during argument unpacking")
						}
						// Track named (string key) vs positional (integer key)
						if key.GetType() == phpv.ZtString {
							seenStringKey = true
						} else if seenStringKey {
							// Positional argument after named argument during unpacking
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								"Cannot use positional argument after named argument during unpacking")
						}
						value, err := obj.CallMethod(ctx, "current")
						if err != nil {
							return nil, err
						}
						result = append(result, &spreadZVal{v: value.Dup(), fromLiteral: true})
						if _, err := obj.CallMethod(ctx, "next"); err != nil {
							return nil, err
						}
					}
					continue
				}
			}
			// Object that does not implement Traversable
			typeName := "stdClass"
			if obj, ok := val.Value().(*phpobj.ZObject); ok {
				typeName = string(obj.GetClass().GetName())
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
		}
		// Non-iterable, non-object spread: TypeError
		typeName := val.GetType().TypeName()
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
	}
	return result, nil
}

// spreadNamedArg wraps a spread arg value with a string key name,
// implementing phpv.NamedArgument so the function call machinery treats it
// as a named parameter.
type spreadNamedArg struct {
	name  phpv.ZString
	inner phpv.Runnable
}

func (s *spreadNamedArg) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return s.inner.Run(ctx)
}

func (s *spreadNamedArg) ArgName() phpv.ZString {
	return s.name
}

func (s *spreadNamedArg) Inner() phpv.Runnable {
	return s.inner
}

func (s *spreadNamedArg) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s: ", s.name)
	if err != nil {
		return err
	}
	return s.inner.Dump(w)
}

// spreadZVal is a Runnable wrapper for pre-evaluated spread argument values.
// It implements Writable so that by-ref parameters can write back to the
// original array element. When fromLiteral is true, no write-back occurs.
type spreadZVal struct {
	v           *phpv.ZVal
	fromLiteral bool // true if from a literal (non-variable), no write-back needed
}

func (s *spreadZVal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return s.v, nil
}

func (s *spreadZVal) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	// Write-back for by-ref parameter passing - update the original array element
	if !s.fromLiteral {
		s.v.Set(value)
	}
	return nil
}

func (s *spreadZVal) Dump(w io.Writer) error {
	_, err := w.Write([]byte("spread_val"))
	return err
}

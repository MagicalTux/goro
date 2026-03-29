package core

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func SpawnCallableParam(ctx phpv.Context, v *phpv.ZVal, paramNo int) (phpv.Callable, error) {
	return spawnCallableInternal(ctx, v, paramNo)
}

func SpawnCallable(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error) {
	return spawnCallableInternal(ctx, v, 1)
}

func spawnCallableInternal(ctx phpv.Context, v *phpv.ZVal, paramNo int) (phpv.Callable, error) {
	switch v.GetType() {
	case phpv.ZtString:
		// name of a method
		s := v.Value().(phpv.ZString)

		if index := strings.Index(string(s), "::"); index >= 0 {
			// handle className::method
			className := s[0:index]
			methodName := s[index+2:]

			var class phpv.ZClass
			classNameLower := className.ToLower()
			if classNameLower == "self" || classNameLower == "parent" || classNameLower == "static" {
				if err := ctx.Deprecated("Use of \"%s\" in callables is deprecated", className, logopt.NoFuncName(true)); err != nil {
					return nil, err
				}
				callerClass := ctx.Class()
				if callerClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use \"%s\" when no class scope is active", className))
				}
				if classNameLower == "parent" {
					class = callerClass.GetParent()
					if class == nil {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" when current class scope has no parent")
					}
				} else if classNameLower == "static" {
					// "static" uses late static binding - resolve to the actual runtime class
					if this := ctx.This(); this != nil {
						class = this.GetClass()
					} else {
						class = callerClass
					}
				} else {
					class = callerClass
				}
			} else {
				var err error
				class, err = ctx.Global().GetClass(ctx, className, true)
				if err != nil {
					// Convert class-not-found errors into a TypeError for callback context.
					// But if the autoloader threw a user exception (not an Error), propagate it.
					if pt, ok := err.(*phperr.PhpThrow); ok {
						if isClassNotFoundError(pt) {
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
								fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", className))
						}
					}
					return nil, err
				}
			}
			member, ok := class.GetMethod(methodName.ToLower())
			if !ok {
				// When inside instance context, prefer __call over __callStatic.
				// Use the actual object class (not the scope class) for the
				// instanceof check, since $this may be a subclass instance.
				if this := ctx.This(); this != nil {
					actualClass := this.GetClass()
					if zo, ok := this.(*phpobj.ZObject); ok {
						actualClass = zo.Class
					}
					if actualClass.InstanceOf(class) {
						if callMethod, hasCall := class.GetMethod("__call"); hasCall {
							wrapper := &magicCallWrapper{
								callMethod: callMethod.Method,
								methodName: methodName,
							}
							return phpv.Bind(wrapper, this), nil
						}
					}
				}
				if callStaticMethod, hasCallStatic := class.GetMethod("__callstatic"); hasCallStatic {
					wrapper := &magicCallWrapper{
						callMethod: callStaticMethod.Method,
						methodName: methodName,
					}
					return phpv.BindClass(wrapper, class, true), nil
				}
				callerFunc := ctx.GetFuncName()
				if callerFunc == "" {
					callerFunc = "call_user_func"
				}
				orNull := ""
				if callerFunc == "spl_autoload_register" {
					orNull = " or null"
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback%s, class %s does not have a method \"%s\"", callerFunc, orNull, className, methodName))
			}

			// Check if the method is abstract (cannot be called directly)
			// This covers both explicit "abstract" methods and interface methods (implicitly abstract)
			if member.Modifiers.Has(phpv.ZAttrAbstract) || member.Empty {
				callerFunc := ctx.GetFuncName()
				if callerFunc == "" {
					callerFunc = "call_user_func"
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback, cannot call abstract method %s::%s()", callerFunc, class.GetName(), member.Name))
			}

			// Check visibility of the method
			callerClass := ctx.Class()
			if member.Modifiers.IsPrivate() {
				declaringClass := class
				if member.Class != nil {
					declaringClass = member.Class
				}
				if callerClass == nil || callerClass.GetName() != declaringClass.GetName() {
					callerFunc := ctx.GetFuncName()
					if callerFunc == "" {
						callerFunc = "call_user_func"
					}
					orNull := ""
					if callerFunc == "spl_autoload_register" {
						orNull = " or null"
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback%s, cannot access private method %s::%s()", callerFunc, paramNo, orNull, class.GetName(), member.Name))
				}
			} else if member.Modifiers.Has(phpv.ZAttrProtected) {
				accessible := false
				if callerClass != nil {
					// Basic hierarchy check: caller is-a target class or vice versa
					accessible = callerClass.InstanceOf(class) || class.InstanceOf(callerClass)
					// Also check via the method's declaring class
					if !accessible && member.Class != nil {
						accessible = callerClass.InstanceOf(member.Class) || member.Class.InstanceOf(callerClass)
					}
					// Also check if caller and target share a common ancestor that declares this method
					// (handles sibling classes, e.g., B1 and B2 both extend A)
					if !accessible && member.Class != nil {
						rootClass := member.Class
						for rootClass.GetParent() != nil {
							if pm, ok := rootClass.GetParent().GetMethod(member.Name); ok && pm.Modifiers.Has(phpv.ZAttrProtected) {
								rootClass = rootClass.GetParent()
							} else {
								break
							}
						}
						if callerClass.InstanceOf(rootClass) {
							accessible = true
						}
					}
				}
				if !accessible {
					callerFunc := ctx.GetFuncName()
					if callerFunc == "" {
						callerFunc = "call_user_func"
					}
					orNull := ""
					if callerFunc == "spl_autoload_register" {
						orNull = " or null"
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback%s, cannot access protected method %s::%s()", callerFunc, paramNo, orNull, class.GetName(), member.Name))
				}
			}

			if member.Modifiers.IsStatic() {
				return phpv.BindClass(member.Method, class, true), nil
			}
			// Non-static method: allow if $this is available and is an instance of the class
			if this := ctx.This(); this != nil && this.GetClass().InstanceOf(class) {
				return phpv.Bind(member.Method, this), nil
			}
			// Non-static method called without instance context — error for spl_autoload_register
			callerFunc := ctx.GetFuncName()
			if callerFunc == "spl_autoload_register" {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback or null, non-static method %s::%s() cannot be called statically", callerFunc, class.GetName(), member.Name))
			}
			return phpv.BindClass(member.Method, class, false), nil
		}

		// PHP 8: scope-dependent functions cannot be called dynamically.
		// Defer the error to call time so that register_shutdown_function etc. succeed.
		sLower := s.ToLower()
		switch sLower {
		case "func_get_args", "func_num_args", "get_defined_vars":
			// These functions take 0 arguments; if called with args, throw ArgumentCountError.
			return &deferredErrorCallable{funcName: string(s), maxArgs: 0}, nil
		case "func_get_arg":
			// Takes exactly 1 argument; check arg count first.
			return &deferredErrorCallable{funcName: string(s), maxArgs: 1}, nil
		case "extract", "compact":
			// These take variable args; only throw "cannot call dynamically".
			return &deferredErrorCallable{funcName: string(s), maxArgs: -1}, nil
		}

		// Language constructs cannot be used as callbacks
		switch sLower {
		case "echo", "print", "isset", "unset", "empty", "list", "eval",
			"die", "exit", "include", "require", "include_once", "require_once":
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, function \"%s\" not found or invalid function name", callerFunc, paramNo, s))
		}

		fn, fnErr := ctx.Global().GetFunction(ctx, s)
		if fnErr != nil {
			// Convert "Call to undefined function" errors to the proper callback error format
			if pt, ok := fnErr.(*phperr.PhpThrow); ok {
				if obj, ok2 := pt.Obj.(phpv.ZObject); ok2 && obj.GetClass().InstanceOf(phpobj.Error) {
					callerFunc := ctx.GetFuncName()
					if callerFunc == "" {
						callerFunc = "call_user_func"
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, function \"%s\" not found or invalid function name", callerFunc, paramNo, s))
				}
			}
			return nil, fnErr
		}
		return fn, nil

	case phpv.ZtArray:
		// array is either:
		// - [$obj, "methodName"]
		// - ["className", "methodName"]
		array := v.Array()
		// PHP requires exactly 2 elements at indices 0 and 1
		has0, _ := array.OffsetExists(ctx, phpv.ZInt(0).ZVal())
		has1, _ := array.OffsetExists(ctx, phpv.ZInt(1).ZVal())
		if !has0 || !has1 {
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			if countable, ok := array.(phpv.ZCountable); !ok || countable.Count(ctx) != 2 {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, array callback must have exactly two members", callerFunc, paramNo))
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, array callback has to contain indices 0 and 1", callerFunc, paramNo))
		}
		firstArg, err := array.OffsetGet(ctx, phpv.ZInt(0))
		if err != nil {
			return nil, err
		}
		methodName, err := array.OffsetGet(ctx, phpv.ZInt(1))
		if err != nil {
			return nil, err
		}

		switch firstArg.GetType() {
		case phpv.ZtString, phpv.ZtObject:
		default:
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, first array member is not a valid class name or object", callerFunc, paramNo))
		}
		if methodName.GetType() != phpv.ZtString {
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, second array member %q is not a valid method", firstArg.GetType().String())
		}

		var class phpv.ZClass
		var instance phpv.ZObject

		if firstArg.GetType() == phpv.ZtString {
			className := firstArg.AsString(ctx)
			classNameLower := className.ToLower()
			if classNameLower == "self" || classNameLower == "parent" || classNameLower == "static" {
				if err := ctx.Deprecated("Use of \"%s\" in callables is deprecated", className, logopt.NoFuncName(true)); err != nil {
					return nil, err
				}
				callerClass := ctx.Class()
				if callerClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use \"%s\" when no class scope is active", className))
				}
				if classNameLower == "parent" {
					class = callerClass.GetParent()
					if class == nil {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" when current class scope has no parent")
					}
				} else if classNameLower == "static" {
					// "static" uses late static binding
					if this := ctx.This(); this != nil {
						class = this.GetClass()
					} else {
						class = callerClass
					}
				} else {
					class = callerClass
				}
			} else {
				class, err = ctx.Global().GetClass(ctx, className, true)
				if err != nil {
					if pt, ok := err.(*phperr.PhpThrow); ok {
						if isClassNotFoundError(pt) {
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", className))
						}
					}
					return nil, err
				}
			}
		} else {
			instance = firstArg.AsObject(ctx)
			class = instance.GetClass()
		}

		origName := methodName.AsString(ctx)
		name := origName.ToLower()
		if index := strings.Index(string(name), "::"); index >= 0 {
			// handle className::method
			className := name[0:index]
			methodNamePart := name[index+2:]
			name = methodNamePart
			// Also update origName to strip the prefix for error messages
			origName = origName[index+2:]

			// Emit deprecated warning about this callable form
			// Use the actual runtime class name for objects, not CurrentClass.
			var displayClassName phpv.ZString
			if firstArg.GetType() == phpv.ZtString {
				displayClassName = firstArg.AsString(ctx)
			} else if zo, ok := instance.(*phpobj.ZObject); ok {
				displayClassName = zo.Class.GetName()
			} else {
				displayClassName = instance.GetClass().GetName()
			}
			origMethodStr := methodName.AsString(ctx)
			ctx.Deprecated("Callables of the form [\"%s\", \"%s\"] are deprecated", displayClassName, origMethodStr, logopt.NoFuncName(true))

			if className == "parent" {
				// For "parent::", resolve relative to the actual runtime class
				// (not CurrentClass), so C->parent = B, not B->parent = A.
				if zo, ok := instance.(*phpobj.ZObject); ok {
					class = zo.Class.GetParent()
				} else {
					class = class.GetParent()
				}
			} else if className == "self" {
				// For "self::", keep as current scope class for method lookup
				// (class already set from GetClass() which returns CurrentClass)
			} else {
				// Look up the specified class
				resolvedClass, err := ctx.Global().GetClass(ctx, className, false)
				if err != nil {
					return nil, err
				}
				class = resolvedClass
			}
		}

		member, ok := class.GetMethod(name)
		if !ok {
			// Check for __invoke via HandleInvoke (e.g., Closure::__invoke)
			if instance != nil && name == "__invoke" && class.Handlers() != nil && class.Handlers().HandleInvoke != nil {
				return phpv.Bind(&invokeWrapper{obj: instance, handler: class.Handlers().HandleInvoke}, instance), nil
			}
			// Check for __call magic method (instance call)
			if instance != nil {
				if callMethod, hasCall := class.GetMethod("__call"); hasCall {
					origMethodName := methodName.AsString(ctx)
					wrapper := &magicCallWrapper{
						callMethod: callMethod.Method,
						methodName: origMethodName,
					}
					return phpv.Bind(wrapper, instance), nil
				}
			}
			// Check for __callStatic or __call (static call with string class name)
			if instance == nil {
				// When inside instance context, prefer __call over __callStatic.
				// Use the actual object class (not the scope class) for the
				// instanceof check, since $this may be a subclass instance.
				if this := ctx.This(); this != nil {
					actualClass := this.GetClass()
					// Also check the underlying Class field for the real runtime class
					if zo, ok := this.(*phpobj.ZObject); ok {
						actualClass = zo.Class
					}
					if actualClass.InstanceOf(class) {
						if callMethod, hasCall := class.GetMethod("__call"); hasCall {
							origMethodName := methodName.AsString(ctx)
							wrapper := &magicCallWrapper{
								callMethod: callMethod.Method,
								methodName: origMethodName,
							}
							return phpv.Bind(wrapper, this), nil
						}
					}
				}
				if callStaticMethod, hasCallStatic := class.GetMethod("__callstatic"); hasCallStatic {
					origMethodName := methodName.AsString(ctx)
					wrapper := &magicCallWrapper{
						callMethod: callStaticMethod.Method,
						methodName: origMethodName,
					}
					return phpv.BindClass(wrapper, class, true), nil
				}
				// Note: __call only applies to instance context, NOT static string context
				// without $this. is_callable(['ClassName', 'method']) should return false
				// if only __call exists (no __callStatic) and we're not in instance context.
			}
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			orNull := ""
			if callerFunc == "spl_autoload_register" {
				orNull = " or null"
			}
			// Use the original method name (preserving case) for the error message
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback%s, class %s does not have a method \"%s\"", callerFunc, orNull, class.GetName(), origName))
		}

		// Check if the method is abstract - abstract methods cannot be called directly
		// This covers both explicit "abstract" methods and interface methods (implicitly abstract)
		if member.Modifiers.Has(phpv.ZAttrAbstract) || member.Empty {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, cannot call abstract method %s::%s()", class.GetName(), member.Name))
		}

		// Check visibility — private/protected methods cannot be called from outside their scope.
		// If the method is not visible but __call exists, fall through to __call.
		callerClass := ctx.Class()
		methodNotVisible := false
		// Use the declaring class for private visibility checks, not the object's class
		declaringClass := class
		if member.Class != nil {
			declaringClass = member.Class
		}
		if member.Modifiers.IsPrivate() {
			// Private: only accessible from the DECLARING class (not subclasses)
			if callerClass == nil || callerClass.GetName() != declaringClass.GetName() {
				methodNotVisible = true
			}
		} else if member.Modifiers.IsProtected() {
			// Protected: only accessible from same class or subclass
			if callerClass == nil || (!callerClass.InstanceOf(declaringClass) && !declaringClass.InstanceOf(callerClass)) {
				// Check if caller and target share a common ancestor (sibling classes)
				visible := false
				if callerClass != nil && member.Class != nil {
					rootClass := member.Class
					for rootClass.GetParent() != nil {
						if pm, ok := rootClass.GetParent().GetMethod(member.Name); ok && pm.Modifiers.Has(phpv.ZAttrProtected) {
							rootClass = rootClass.GetParent()
						} else {
							break
						}
					}
					if callerClass.InstanceOf(rootClass) {
						visible = true
					}
				}
				if !visible {
					methodNotVisible = true
				}
			}
		}
		if methodNotVisible {
			// Check for __call magic method before throwing visibility error
			if instance != nil {
				if callMethod, hasCall := class.GetMethod("__call"); hasCall {
					origMethodName := methodName.AsString(ctx)
					wrapper := &magicCallWrapper{
						callMethod: callMethod.Method,
						methodName: origMethodName,
					}
					return phpv.Bind(wrapper, instance), nil
				}
			}
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			orNull := ""
			if callerFunc == "spl_autoload_register" {
				orNull = " or null"
			}
			if member.Modifiers.IsPrivate() {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback%s, cannot access private method %s::%s()", callerFunc, orNull, class.GetName(), member.Name))
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback%s, cannot access protected method %s::%s()", callerFunc, orNull, class.GetName(), member.Name))
		}

		if instance != nil {
			// Static methods should not receive $this even when called on an instance
			if member.Modifiers.IsStatic() {
				return phpv.BindClass(member.Method, class, true), nil
			}
			method := phpv.Bind(member.Method, instance)
			return method, nil
		}
		// Non-static method with class name string (no instance): throw TypeError
		if !member.Modifiers.IsStatic() {
			// Check if $this is available and is an instance of the class
			if this := ctx.This(); this != nil && this.GetClass().InstanceOf(class) {
				return phpv.Bind(member.Method, this), nil
			}
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			orNull := ""
			if callerFunc == "spl_autoload_register" {
				orNull = " or null"
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback%s, non-static method %s::%s() cannot be called statically", callerFunc, orNull, class.GetName(), member.Name))
		}
		return phpv.BindClass(member.Method, class, true), nil

	case phpv.ZtCallable:
		if c, ok := v.Value().(phpv.Callable); ok {
			return c, nil
		}
		return nil, ctx.Errorf("Argument passed must be callable, %q given", v.GetType().String())

	case phpv.ZtObject:
		object := v.AsObject(ctx)

		// For Closure objects, use the opaque ZClosure directly
		if opaque := object.GetOpaque(compiler.Closure); opaque != nil {
			switch z := opaque.(type) {
			case *compiler.ZClosure:
				return z, nil
			case phpv.Callable:
				return z, nil
			}
		}

		if f, ok := object.GetClass().GetMethod("__invoke"); ok {
			method := phpv.Bind(f.Method, object)
			return method, nil
		}

		fallthrough
	default:
		callerFunc := ctx.GetFuncName()
		if callerFunc == "" {
			callerFunc = "call_user_func"
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, no array or string given", callerFunc, paramNo))
	}
}

// deferredErrorCallable is a callable that always throws an error when called.
// Used for scope-dependent functions (get_defined_vars, func_get_args, etc.)
// that cannot be called dynamically - the error is deferred to call time.
type deferredErrorCallable struct {
	phpv.CallableVal
	funcName string
	maxArgs  int // expected argument count (0 = none, -1 = unchecked)
}

func (d *deferredErrorCallable) Name() string {
	return d.funcName
}

func (d *deferredErrorCallable) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// If the function has a fixed argument count and was called with wrong number of args,
	// throw ArgumentCountError (mirrors PHP behavior for built-in functions).
	if d.maxArgs >= 0 && len(args) > d.maxArgs {
		s := ""
		if d.maxArgs != 1 {
			s = "s"
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s() expects exactly %d argument%s, %d given", d.funcName, d.maxArgs, s, len(args)))
	}
	return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot call %s() dynamically", d.funcName))
}

// invokeWrapper wraps an object's HandleInvoke handler as a Callable.
type invokeWrapper struct {
	phpv.CallableVal
	obj     phpv.ZObject
	handler func(phpv.Context, phpv.ZObject, []phpv.Runnable) (*phpv.ZVal, error)
}

func (w *invokeWrapper) Name() string {
	return "__invoke"
}

func (w *invokeWrapper) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Convert []*ZVal to []Runnable using zvalRunnable wrapper
	runnables := make([]phpv.Runnable, len(args))
	for i, a := range args {
		runnables[i] = &zvalRunnable{v: a}
	}
	// When calling __invoke via method dispatch (e.g., call_user_func([$f, '__invoke'], ...)),
	// suppress "called in" suffix on type errors (PHP behavior).
	ctx.Global().SetNextCallSuppressCalledIn(true)
	res, err := w.handler(ctx, w.obj, runnables)
	ctx.Global().SetNextCallSuppressCalledIn(false)
	return res, err
}

// zvalRunnable wraps a *ZVal as a Runnable (for passing pre-evaluated values as Runnable args)
type zvalRunnable struct {
	v *phpv.ZVal
}

func (r *zvalRunnable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return r.v, nil
}

func (r *zvalRunnable) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%v", r.v)
	return err
}

// IsPreEvaluatedArg marks zvalRunnable as a pre-evaluated argument (from call_user_func).
// When passed to a by-reference parameter, this causes the call infrastructure to emit
// a Warning ("FuncName(): Argument #N must be passed by reference, value given")
// rather than a Notice or Fatal Error, matching PHP's call_user_func behavior.
func (r *zvalRunnable) IsPreEvaluatedArg() {}

// magicCallWrapper wraps a __call magic method to be used as a Callable.
// When called, it packages the arguments into the __call($methodName, $args) format.
type magicCallWrapper struct {
	phpv.CallableVal
	callMethod phpv.Callable
	methodName phpv.ZString
}

func (w *magicCallWrapper) Name() string {
	return string(w.methodName)
}

func (w *magicCallWrapper) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Build args array for __call
	a := phpv.NewZArray()
	for _, arg := range args {
		a.OffsetSet(ctx, nil, arg.Dup())
	}
	callArgs := []*phpv.ZVal{w.methodName.ZVal(), a.ZVal()}
	return w.callMethod.Call(ctx, callArgs)
}

// isClassNotFoundError checks if a PhpThrow represents a class-not-found Error
// (as opposed to a user exception thrown from an autoloader).
func isClassNotFoundError(pt *phperr.PhpThrow) bool {
	if pt.Obj == nil {
		return false
	}
	obj, ok := pt.Obj.(phpv.ZObject)
	if !ok {
		return false
	}
	// Class-not-found errors are Error instances, not Exception instances
	return obj.GetClass().InstanceOf(phpobj.Error)
}

// CallbackErrorReason extracts the reason portion from a SpawnCallable error.
// SpawnCallable errors are typically formatted as:
//   "funcName(): Argument #N ($callback) must be a valid callback, <reason>"
// This function returns just the "<reason>" part for re-wrapping by callers.
// If the error is not a callback error, it returns the full error string.
func CallbackErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Try to extract from PhpThrow message
	if pt, ok := err.(*phperr.PhpThrow); ok {
		if obj, ok2 := pt.Obj.(phpv.ZObject); ok2 {
			if msgVal := obj.HashTable().GetString("message"); msgVal != nil {
				msg = msgVal.String()
			}
		}
	}
	// Look for "must be a valid callback, " and extract everything after it
	const marker = "must be a valid callback, "
	if idx := strings.Index(msg, marker); idx >= 0 {
		return msg[idx+len(marker):]
	}
	return msg
}

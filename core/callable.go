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

			class, err := ctx.Global().GetClass(ctx, className, true)
			if err != nil {
				// Convert class-not-found errors into a TypeError for callback context
				if _, ok := err.(*phperr.PhpThrow); ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", className))
				}
				return nil, err
			}
			member, ok := class.GetMethod(methodName.ToLower())
			if !ok {
				callerFunc := ctx.GetFuncName()
				if callerFunc == "" {
					callerFunc = "call_user_func"
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback, class \"%s\" does not have a method \"%s\"", callerFunc, className, methodName))
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
					scope := "global scope"
					if callerClass != nil {
						scope = "scope " + string(callerClass.GetName())
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, cannot access private method %s::%s() from %s", callerFunc, paramNo, class.GetName(), member.Name, scope))
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
				}
				if !accessible {
					callerFunc := ctx.GetFuncName()
					if callerFunc == "" {
						callerFunc = "call_user_func"
					}
					scope := "global scope"
					if callerClass != nil {
						scope = "scope " + string(callerClass.GetName())
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, cannot access protected method %s::%s() from %s", callerFunc, paramNo, class.GetName(), member.Name, scope))
				}
			}

			if member.Modifiers.IsStatic() {
				return phpv.BindClass(member.Method, class, true), nil
			}
			// Non-static method: allow if $this is available and is an instance of the class
			if this := ctx.This(); this != nil && this.GetClass().InstanceOf(class) {
				return phpv.Bind(member.Method, this), nil
			}
			// Non-static method called without instance context — deprecated in PHP 8
			return phpv.BindClass(member.Method, class, false), nil
		}

		// PHP 8: scope-dependent functions cannot be called dynamically
		sLower := s.ToLower()
		if sLower == "extract" || sLower == "compact" || sLower == "get_defined_vars" || sLower == "func_get_args" || sLower == "func_get_arg" || sLower == "func_num_args" {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot call %s() dynamically", s))
		}

		return ctx.Global().GetFunction(ctx, s)

	case phpv.ZtArray:
		// array is either:
		// - [$obj, "methodName"]
		// - ["className", "methodName"]
		array := v.Array()
		// PHP requires exactly 2 elements at indices 0 and 1
		has0, _ := array.OffsetExists(ctx, phpv.ZInt(0).ZVal())
		has1, _ := array.OffsetExists(ctx, phpv.ZInt(1).ZVal())
		if !has0 || !has1 {
			if countable, ok := array.(phpv.ZCountable); !ok || countable.Count(ctx) != 2 {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Array callback must have exactly two elements")
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Array callback has to contain indices 0 and 1")
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
			if classNameLower == "self" || classNameLower == "parent" {
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
				} else {
					class = callerClass
				}
			} else {
				class, err = ctx.Global().GetClass(ctx, className, true)
				if err != nil {
					if _, ok := err.(*phperr.PhpThrow); ok {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", className))
					}
					return nil, err
				}
			}
		} else {
			instance = firstArg.AsObject(ctx)
			class = instance.GetClass()
		}

		name := methodName.AsString(ctx).ToLower()
		if index := strings.Index(string(name), "::"); index >= 0 {
			// handle className::method
			className := name[0:index]
			methodNamePart := name[index+2:]
			name = methodNamePart

			// Emit deprecated warning about this callable form
			var displayClassName phpv.ZString
			if firstArg.GetType() == phpv.ZtString {
				displayClassName = firstArg.AsString(ctx)
			} else {
				displayClassName = instance.GetClass().GetName()
			}
			origMethodStr := methodName.AsString(ctx)
			ctx.Deprecated("Callables of the form [\"%s\", \"%s\"] are deprecated", displayClassName, origMethodStr, logopt.NoFuncName(true))

			if className == "parent" {
				class = class.GetParent()
			} else if className == "self" {
				// keep class as-is
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
				if callStaticMethod, hasCallStatic := class.GetMethod("__callstatic"); hasCallStatic {
					origMethodName := methodName.AsString(ctx)
					wrapper := &magicCallWrapper{
						callMethod: callStaticMethod.Method,
						methodName: origMethodName,
					}
					return phpv.BindClass(wrapper, class, true), nil
				}
				if callMethod, hasCall := class.GetMethod("__call"); hasCall {
					origMethodName := methodName.AsString(ctx)
					wrapper := &magicCallWrapper{
						callMethod: callMethod.Method,
						methodName: origMethodName,
					}
					return phpv.BindClass(wrapper, class, false), nil
				}
			}
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, method not found: %q", methodName)
		}

		// Check if the method is abstract - abstract methods cannot be called directly
		if member.Modifiers.Has(phpv.ZAttrAbstract) {
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
				methodNotVisible = true
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
			if member.Modifiers.IsPrivate() {
				callerFunc := ctx.GetFuncName()
				if callerFunc == "" {
					callerFunc = "call_user_func"
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback, cannot access private method %s::%s()", callerFunc, class.GetName(), member.Name))
			}
			callerFunc := ctx.GetFuncName()
			if callerFunc == "" {
				callerFunc = "call_user_func"
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Argument #1 ($callback) must be a valid callback, cannot access protected method %s::%s()", callerFunc, class.GetName(), member.Name))
		}

		if instance != nil {
			method := phpv.Bind(member.Method, instance)
			return method, nil
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
			fmt.Sprintf("%s(): Argument #%d ($callback) must be a valid callback, no array, string, or closure given", callerFunc, paramNo))
	}
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
	return w.handler(ctx, w.obj, runnables)
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

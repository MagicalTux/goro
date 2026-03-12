package core

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func SpawnCallable(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error) {
	switch v.GetType() {
	case phpv.ZtString:
		// name of a method
		s := v.Value().(phpv.ZString)

		if index := strings.Index(string(s), "::"); index >= 0 {
			// handle className::method
			className := s[0:index]
			methodName := s[index+2:]

			class, err := ctx.Global().GetClass(ctx, className, false)
			if err != nil {
				return nil, err
			}
			member, ok := class.GetMethod(methodName.ToLower())
			if !ok || !member.Modifiers.IsStatic() {
				return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, class MyClass does not have a method %q", methodName)
			}

			return phpv.BindClass(member.Method, class, true), nil
		}

		return ctx.Global().GetFunction(ctx, s)

	case phpv.ZtArray:
		// array is either:
		// - [$obj, "methodName"]
		// - ["className", "methodName"]
		array := v.Array()
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
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, first array member %q is not a valid class name or object", firstArg.GetType().String())
		}
		if methodName.GetType() != phpv.ZtString {
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, second array member %q is not a valid method", firstArg.GetType().String())
		}

		var class phpv.ZClass
		var instance phpv.ZObject

		if firstArg.GetType() == phpv.ZtString {
			class, err = ctx.Global().GetClass(ctx, firstArg.AsString(ctx), false)
			if err != nil {
				return nil, err
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
			// Check for __call magic method
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
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, method not found: %q", methodName)
		}

		// Check if the method is abstract - abstract methods cannot be called directly
		if member.Modifiers.Has(phpv.ZAttrAbstract) {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("call_user_func(): Argument #1 ($callback) must be a valid callback, cannot call abstract method %s::%s()", class.GetName(), member.Name))
		}

		if instance != nil {
			method := phpv.Bind(member.Method, instance)
			return method, nil
		}
		return phpv.BindClass(member.Method, class, true), nil

	case phpv.ZtObject:
		object := v.AsObject(ctx)
		if f, ok := object.GetClass().GetMethod("__invoke"); ok {
			method := phpv.Bind(f.Method, object)
			return method, nil
		}

		if z, ok := object.GetOpaque(compiler.Closure).(*compiler.ZClosure); ok {
			return z, nil
		}

		fallthrough
	default:
		return nil, ctx.Errorf("Argument passed must be callable, %q given", v.GetType().String())
	}
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

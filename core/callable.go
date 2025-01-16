package core

import (
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
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

			return member.Method, nil
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
			class, err := ctx.Global().GetClass(ctx, firstArg.AsString(ctx), false)
			if err != nil {
				return nil, err
			}
			instance, err = phpobj.NewZObject(ctx, class)
			if err != nil {
				return nil, err
			}
		} else {
			instance = firstArg.AsObject(ctx)
		}

		class = instance.GetClass()

		name := methodName.AsString(ctx).ToLower()
		if index := strings.Index(string(name), "::"); index >= 0 {
			// handle className::method
			className := name[0:index]
			methodName := name[index+2:]
			name = methodName
			if className == "parent" {
				class = class.GetParent()
			} else if className != "self" {
				return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, second array member %q is not a valid method", className)
			}
		}

		member, ok := class.GetMethod(name)
		if !ok {
			return nil, ctx.Errorf("Argument #1 ($callback) must be a valid callback, method not found: %q", methodName)
		}

		method := phpv.Bind(member.Method, instance)
		return method, nil

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

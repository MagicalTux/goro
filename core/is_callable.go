package core

import (
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool is_callable ( mixed $value [, bool $syntax_only = false [, string &$callable_name = null ]] )
func fncIsCallable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var value *phpv.ZVal
	var syntaxOnly Optional[phpv.ZBool]
	var callableName OptionalRef[*phpv.ZVal]

	_, err := Expand(ctx, args, &value, &syntaxOnly, &callableName)
	if err != nil {
		return nil, err
	}

	syntaxOnlyBool := bool(syntaxOnly.GetOrDefault(phpv.ZBool(false)))

	if syntaxOnlyBool {
		// Just check if the syntax looks like a callable
		ok, name := isCallableSyntax(ctx, value)
		if callableName.HasArg() {
			callableName.Set(ctx, phpv.ZString(name).ZVal())
		}
		return phpv.ZBool(ok).ZVal(), nil
	}

	// Use the parent context for SpawnCallable so that visibility checks
	// use the calling scope's class (not the is_callable function scope).
	callerCtx := ctx.Parent(1)
	if callerCtx == nil {
		callerCtx = ctx
	}
	callable, resolveErr := SpawnCallable(callerCtx, value)
	if resolveErr != nil || callable == nil {
		// Not callable, but still set callable_name if requested
		if callableName.HasArg() {
			_, name := isCallableSyntax(ctx, value)
			callableName.Set(ctx, phpv.ZString(name).ZVal())
		}
		return phpv.ZFalse.ZVal(), nil
	}

	if callableName.HasArg() {
		displayName := phpv.CallableDisplayName(callable)
		// For array callables like [$obj, "method"], use the syntax-derived name
		// when the callable display name is incomplete (e.g., NativeMethod has empty name)
		if value.GetType() == phpv.ZtArray && strings.HasSuffix(displayName, "::") {
			_, syntaxName := isCallableSyntax(ctx, value)
			if syntaxName != "" {
				displayName = syntaxName
			}
		}
		callableName.Set(ctx, phpv.ZString(displayName).ZVal())
	}
	return phpv.ZTrue.ZVal(), nil
}

// closureName extracts the display name from a closure opaque value.
func closureName(opaque interface{}) string {
	if namer, ok := opaque.(interface{ Name() string }); ok {
		return namer.Name()
	}
	return "Closure::__invoke"
}

// isCallableSyntax checks if the value has a valid callable syntax and returns
// whether it is syntactically valid and the name representation.
func isCallableSyntax(ctx phpv.Context, value *phpv.ZVal) (bool, string) {
	switch value.GetType() {
	case phpv.ZtString:
		s := string(value.AsString(ctx))
		if s == "" {
			return false, ""
		}
		return true, s
	case phpv.ZtArray:
		arr := value.Array()
		if arr == nil {
			return false, ""
		}
		first, err1 := arr.OffsetGet(ctx, phpv.ZInt(0))
		second, err2 := arr.OffsetGet(ctx, phpv.ZInt(1))
		if err1 != nil || err2 != nil || first == nil || second == nil {
			return false, ""
		}
		if second.GetType() != phpv.ZtString {
			return false, ""
		}
		methodName := string(second.AsString(ctx))

		switch first.GetType() {
		case phpv.ZtString:
			className := string(first.AsString(ctx))
			return true, className + "::" + methodName
		case phpv.ZtObject:
			obj := first.AsObject(ctx)
			className := string(obj.GetClass().GetName())
			return true, className + "::" + methodName
		default:
			return false, ""
		}
	case phpv.ZtObject:
		obj := value.AsObject(ctx)
		if obj == nil {
			return false, ""
		}
		// Check if it's a Closure - return the closure's actual name
		if opaque := obj.GetOpaque(compiler.Closure); opaque != nil {
			return true, closureName(opaque)
		}
		// Check for __invoke method
		if _, ok := obj.GetClass().GetMethod("__invoke"); ok {
			return true, string(obj.GetClass().GetName()) + "::__invoke"
		}
		return false, ""
	case phpv.ZtCallable:
		if c, ok := value.Value().(phpv.Callable); ok {
			return true, c.Name()
		}
		return false, ""
	default:
		// Try to get a string representation for the name
		if value.GetType() == phpv.ZtNull {
			return false, ""
		}
		s := strings.TrimSpace(string(value.AsString(ctx)))
		return false, s
	}
}

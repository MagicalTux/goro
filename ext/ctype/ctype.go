package ctype

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/util"
)

// simple implementation of ctype methods
// ctypeArg returns the string to check and a bool indicating whether the ctype
// function should proceed (false means return false immediately, e.g. for non-string/non-numeric types).
func ctypeArg(ctx phpv.Context, args []*phpv.ZVal) (string, bool, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return "", false, err
	}

	// PHP 8.1+: passing non-string to ctype_* functions is deprecated.
	switch v.GetType() {
	case phpv.ZtString:
		// Strings are the expected type, no deprecation
		return string(v.Value().(phpv.ZString)), true, nil

	case phpv.ZtInt:
		i := v.Value().(phpv.ZInt)
		if err := ctx.Deprecated("Argument of type int will be interpreted as string in the future"); err != nil {
			return "", false, err
		}
		if i >= -128 && i <= 255 {
			// Legacy behavior: treat as character code
			return string([]byte{byte(i)}), true, nil
		}
		// Outside range: convert to string representation
		s, _ := v.As(ctx, phpv.ZtString)
		return string(s.Value().(phpv.ZString)), true, nil

	case phpv.ZtFloat:
		if err := ctx.Deprecated("Argument of type float will be interpreted as string in the future"); err != nil {
			return "", false, err
		}
		// Floats always return false after deprecation notice
		return "", false, nil

	case phpv.ZtObject:
		// For objects, the type name is the class name
		obj := v.Value().(phpv.ZObject)
		typeName := string(obj.GetClass().GetName())
		if err := ctx.Deprecated("Argument of type %s will be interpreted as string in the future", typeName); err != nil {
			return "", false, err
		}
		return "", false, nil

	default:
		// null, bool, array, resource - all deprecated and return false
		typeName := v.GetType().TypeName()
		if err := ctx.Deprecated("Argument of type %s will be interpreted as string in the future", typeName); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
}

// > func bool ctype_alnum ( string $text )
func ctypeAlnum(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeAlnum(v)).ZVal(), nil
}

// > func bool ctype_alpha ( string $text )
func ctypeAlpha(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeAlpha(v)).ZVal(), nil
}

// > func bool ctype_cntrl ( string $text )
func ctypeCntrl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeCntrl(v)).ZVal(), nil
}

// > func bool ctype_digit ( string $text )
func ctypeDigit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeDigit(v)).ZVal(), nil
}

// > func bool ctype_graph ( string $text )
func ctypeGraph(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeGraph(v)).ZVal(), nil
}

// > func bool ctype_lower ( string $text )
func ctypeLower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeLower(v)).ZVal(), nil
}

// > func bool ctype_print ( string $text )
func ctypePrint(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypePrint(v)).ZVal(), nil
}

// > func bool ctype_punct ( string $text )
func ctypePunct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypePunct(v)).ZVal(), nil
}

// > func bool ctype_space ( string $text )
func ctypeSpace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeSpace(v)).ZVal(), nil
}

// > func bool ctype_upper ( string $text )
func ctypeUpper(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeUpper(v)).ZVal(), nil
}

// > func bool ctype_xdigit ( string $text )
func ctypeXdigit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(util.CtypeXdigit(v)).ZVal(), nil
}

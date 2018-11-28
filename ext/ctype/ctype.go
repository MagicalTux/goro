package ctype

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/util"
)

// simple implementation of ctype methods
func ctypeArg(ctx phpv.Context, args []*phpv.ZVal) (string, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return "", err
	}

	// convert
	if v.GetType() == phpv.ZtInt {
		i := v.Value().(phpv.ZInt)
		if i >= -128 && i <= 255 {
			return string([]byte{byte(i)}), nil
		}
	}

	v, err = v.As(ctx, phpv.ZtString)
	if err != nil {
		return "", err
	}
	return string(v.Value().(phpv.ZString)), nil
}

//> func bool ctype_alnum ( string $text )
func ctypeAlnum(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeAlnum(v)).ZVal(), nil
}

//> func bool ctype_alpha ( string $text )
func ctypeAlpha(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeAlpha(v)).ZVal(), nil
}

//> func bool ctype_cntrl ( string $text )
func ctypeCntrl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeCntrl(v)).ZVal(), nil
}

//> func bool ctype_digit ( string $text )
func ctypeDigit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeDigit(v)).ZVal(), nil
}

//> func bool ctype_graph ( string $text )
func ctypeGraph(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeGraph(v)).ZVal(), nil
}

//> func bool ctype_lower ( string $text )
func ctypeLower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeLower(v)).ZVal(), nil
}

//> func bool ctype_print ( string $text )
func ctypePrint(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypePrint(v)).ZVal(), nil
}

//> func bool ctype_punct ( string $text )
func ctypePunct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypePunct(v)).ZVal(), nil
}

//> func bool ctype_space ( string $text )
func ctypeSpace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeSpace(v)).ZVal(), nil
}

//> func bool ctype_upper ( string $text )
func ctypeUpper(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeUpper(v)).ZVal(), nil
}

//> func bool ctype_xdigit ( string $text )
func ctypeXdigit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return phpv.ZBool(util.CtypeXdigit(v)).ZVal(), nil
}

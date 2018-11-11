package ctype

import "github.com/MagicalTux/gophp/core"

// simple implementation of ctype methods
func ctypeArg(ctx core.Context, args []*core.ZVal) (string, error) {
	var v *core.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return "", err
	}

	// convert
	if v.GetType() == core.ZtInt {
		i := v.Value().(core.ZInt)
		if i >= -128 && i <= 255 {
			return string([]byte{byte(i)}), nil
		}
	}

	v, err = v.As(ctx, core.ZtString)
	if err != nil {
		return "", err
	}
	return string(v.Value().(core.ZString)), nil
}

//> func bool ctype_alnum ( string $text )
func ctypeAlnum(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeAlnum(v)).ZVal(), nil
}

//> func bool ctype_alpha ( string $text )
func ctypeAlpha(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeAlpha(v)).ZVal(), nil
}

//> func bool ctype_cntrl ( string $text )
func ctypeCntrl(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeCntrl(v)).ZVal(), nil
}

//> func bool ctype_digit ( string $text )
func ctypeDigit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeDigit(v)).ZVal(), nil
}

//> func bool ctype_graph ( string $text )
func ctypeGraph(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeGraph(v)).ZVal(), nil
}

//> func bool ctype_lower ( string $text )
func ctypeLower(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeLower(v)).ZVal(), nil
}

//> func bool ctype_print ( string $text )
func ctypePrint(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypePrint(v)).ZVal(), nil
}

//> func bool ctype_punct ( string $text )
func ctypePunct(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypePunct(v)).ZVal(), nil
}

//> func bool ctype_space ( string $text )
func ctypeSpace(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeSpace(v)).ZVal(), nil
}

//> func bool ctype_upper ( string $text )
func ctypeUpper(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeUpper(v)).ZVal(), nil
}

//> func bool ctype_xdigit ( string $text )
func ctypeXdigit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	v, err := ctypeArg(ctx, args)
	if err != nil {
		return nil, err
	}

	return core.ZBool(core.CtypeXdigit(v)).ZVal(), nil
}

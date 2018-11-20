package standard

import "github.com/MagicalTux/gophp/core"

//> func bool boolval ( mixed $var )
func fncBoolval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, core.ZtBool)
}

//> func float doubleval ( mixed $var )
func fncDoubleval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return fncFloatval(ctx, args)
}

//> func float floatval ( mixed $var )
func fncFloatval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, core.ZtFloat)
}

//> func int intval ( mixed $var [, int $base = 10 ] )
func fncIntval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZVal
	var base *core.ZInt
	_, err := core.Expand(ctx, args, &v, &base)
	if err != nil {
		return nil, err
	}

	// TODO handle base
	return v.As(ctx, core.ZtInt)
}

//> func string strval ( mixed $var )
func fncStrval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, core.ZtString)
}

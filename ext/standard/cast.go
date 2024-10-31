package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool boolval ( mixed $var )
func fncBoolval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtBool)
}

// > func float doubleval ( mixed $var )
func fncDoubleval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncFloatval(ctx, args)
}

// > func float floatval ( mixed $var )
func fncFloatval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtFloat)
}

// > func int intval ( mixed $var [, int $base = 10 ] )
func fncIntval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	var base *phpv.ZInt
	_, err := core.Expand(ctx, args, &v, &base)
	if err != nil {
		return nil, err
	}

	// TODO handle base
	return v.As(ctx, phpv.ZtInt)
}

// > func string strval ( mixed $var )
func fncStrval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtString)
}

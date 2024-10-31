package gmp

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string gmp_strval ( GMP $gmpnumber [, int $base = 10 ] )
func gmpStrval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal
	var base *phpv.ZInt

	_, err := core.Expand(ctx, args, &gmpnum, &base)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, gmpnum)
	if err != nil {
		return nil, err
	}

	if base == nil {
		base = new(phpv.ZInt)
		*base = 10
	}

	return phpv.ZString(i.Text(int(*base))).ZVal(), nil
}

// > func int gmp_intval ( GMP $gmpnumber )
func gmpIntval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal

	_, err := core.Expand(ctx, args, &gmpnum)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, gmpnum)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(i.Int64()).ZVal(), nil
}

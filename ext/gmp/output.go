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

	b := int(*base)
	// Validate base: must be 2-62 or -36 to -2
	if (b < 2 || b > 62) && (b < -36 || b > -2) {
		return nil, ctx.FuncErrorf("gmp_strval(): Argument #2 ($base) must be between 2 and 62, or between -2 and -36")
	}

	// Negative base means uppercase letters
	if b < 0 {
		b = -b
	}

	return phpv.ZString(i.Text(b)).ZVal(), nil
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

package gmp

import "github.com/MagicalTux/gophp/core"

//> func string gmp_strval ( GMP $gmpnumber [, int $base = 10 ] )
func gmpStrval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var gmpnum *core.ZVal
	var base *core.ZInt

	_, err := core.Expand(ctx, args, &gmpnum, &base)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, gmpnum)
	if err != nil {
		return nil, err
	}

	if base == nil {
		base = new(core.ZInt)
		*base = 10
	}

	return core.ZString(i.Text(int(*base))).ZVal(), nil
}

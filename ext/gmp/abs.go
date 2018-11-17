package gmp

import (
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

//> func GMP gmp_abs ( GMP $a )
func gmpAbs(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var gmpnum *core.ZVal

	_, err := core.Expand(ctx, args, &gmpnum)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, gmpnum)
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Abs(i)

	return returnInt(ctx, r)
}

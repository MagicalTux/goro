package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_abs ( GMP $a )
func gmpAbs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal

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

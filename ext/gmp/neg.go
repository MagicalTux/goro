package gmp

import (
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

//> func GMP gmp_neg ( GMP $a )
func gmpNeg(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var a *core.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Neg(i)

	return returnInt(ctx, r)
}

package gmp

import (
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

//> func GMP gmp_add ( GMP $a , GMP $b )
func gmpAdd(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var a, b *core.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}
	ib, err := readInt(ctx, b)
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Add(ia, ib)

	return returnInt(ctx, r)
}

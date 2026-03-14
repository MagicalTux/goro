package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP|false gmp_invert ( GMP $a , GMP $b )
func gmpInvert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

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
	r.ModInverse(ia, ib)

	if r == nil {
		// No inverse exists
		return phpv.ZFalse.ZVal(), nil
	}

	return returnInt(ctx, r)
}

package gmp

import (
	"math/big"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func GMP gmp_mul ( GMP $a , GMP $b )
func gmpMul(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_mul", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_mul", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Mul(ia, ib)

	return returnInt(ctx, r)
}

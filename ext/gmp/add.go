package gmp

import (
	"math/big"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func GMP gmp_add ( GMP $a , GMP $b )
func gmpAdd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_add", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_add", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Add(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_sub ( GMP $a , GMP $b )
func gmpSub(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_sub", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_sub", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Sub(ia, ib)

	return returnInt(ctx, r)
}

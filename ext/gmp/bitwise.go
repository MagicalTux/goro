package gmp

import (
	"math/big"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func GMP gmp_and ( GMP $a , GMP $b )
func gmpAnd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_and", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_and", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.And(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_or ( GMP $a , GMP $b )
func gmpOr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_or", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_or", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Or(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_xor ( GMP $a , GMP $b )
func gmpXor(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_xor", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_xor", 2, "num2")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Xor(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_com ( GMP $a )
func gmpCom(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, a, "gmp_com", 1, "num")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Not(i)

	return returnInt(ctx, r)
}

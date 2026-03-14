package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_div_q ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivQ(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if ib.Sign() == 0 {
		return nil, errors.New("Division by zero")
	}

	r := &big.Int{}
	r.Quo(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_div_r ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivR(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if ib.Sign() == 0 {
		return nil, errors.New("Division by zero")
	}

	r := &big.Int{}
	r.Rem(ia, ib)

	return returnInt(ctx, r)
}

// > func GMP gmp_mod ( GMP $a , GMP $b )
func gmpMod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if ib.Sign() == 0 {
		return nil, errors.New("Division by zero")
	}

	r := &big.Int{}
	r.Mod(ia, ib)

	return returnInt(ctx, r)
}

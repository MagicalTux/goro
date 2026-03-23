package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_q(): Argument #2 ($num2) Division by zero")
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_r(): Argument #2 ($num2) Division by zero")
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_mod(): Argument #2 ($num2) Division by zero")
	}

	r := &big.Int{}
	r.Mod(ia, ib)

	return returnInt(ctx, r)
}

// gmp_div is an alias for gmp_div_q but with its own error message
func gmpDiv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div(): Argument #2 ($num2) Division by zero")
	}

	r := &big.Int{}
	r.Quo(ia, ib)

	return returnInt(ctx, r)
}

// > func array gmp_div_qr ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivQR(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_qr(): Argument #2 ($num2) Division by zero")
	}

	q := &big.Int{}
	r := &big.Int{}
	q.QuoRem(ia, ib, r)

	qz, err := returnInt(ctx, q)
	if err != nil {
		return nil, err
	}
	rz, err := returnInt(ctx, r)
	if err != nil {
		return nil, err
	}

	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, nil, qz)
	arr.OffsetSet(ctx, nil, rz)

	return arr.ZVal(), nil
}

// > func GMP gmp_divexact ( GMP $a , GMP $b )
func gmpDivexact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_divexact(): Argument #2 ($num2) Division by zero")
	}

	r := &big.Int{}
	r.Quo(ia, ib) // exact division (same as Quo for integers)

	return returnInt(ctx, r)
}

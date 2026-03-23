package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_pow ( GMP $base , int $exp )
func gmpPow(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var base *phpv.ZVal
	var exp phpv.ZInt

	_, err := core.Expand(ctx, args, &base, &exp)
	if err != nil {
		return nil, err
	}

	ibase, err := readInt(ctx, base)
	if err != nil {
		return nil, err
	}

	if exp < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_pow(): Argument #2 ($exponent) must be greater than or equal to 0")
	}

	r := &big.Int{}
	r.Exp(ibase, big.NewInt(int64(exp)), nil)

	return returnInt(ctx, r)
}

// > func GMP gmp_powm ( GMP $base , GMP $exp , GMP $mod )
func gmpPowm(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var base, exp, mod *phpv.ZVal

	_, err := core.Expand(ctx, args, &base, &exp, &mod)
	if err != nil {
		return nil, err
	}

	ibase, err := readInt(ctx, base)
	if err != nil {
		return nil, err
	}
	iexp, err := readInt(ctx, exp)
	if err != nil {
		return nil, err
	}
	imod, err := readInt(ctx, mod)
	if err != nil {
		return nil, err
	}

	if imod.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_powm(): Argument #3 ($modulus) Division by zero")
	}

	// For negative exponents, we need to compute the modular inverse first
	if iexp.Sign() < 0 {
		// base^(-exp) mod m = (base^(-1))^exp mod m
		inv := new(big.Int).ModInverse(ibase, imod)
		if inv == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_powm(): Argument #1 ($num) is not invertible modulo argument #3 ($modulus)")
		}
		posExp := new(big.Int).Neg(iexp)
		r := new(big.Int).Exp(inv, posExp, imod)
		return returnInt(ctx, r)
	}

	r := &big.Int{}
	r.Exp(ibase, iexp, imod)

	return returnInt(ctx, r)
}

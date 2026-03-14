package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
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
		return nil, errors.New("Negative exponent is not supported")
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
		return nil, errors.New("Division by zero")
	}

	r := &big.Int{}
	r.Exp(ibase, iexp, imod)

	return returnInt(ctx, r)
}

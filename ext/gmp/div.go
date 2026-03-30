package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// divWithRound performs integer division with rounding mode:
// mode 0 (GMP_ROUND_ZERO): truncate toward zero (like Quo)
// mode 1 (GMP_ROUND_PLUSINF): round toward +infinity (ceiling)
// mode 2 (GMP_ROUND_MINUSINF): round toward -infinity (floor)
func divWithRound(a, b *big.Int, mode int) *big.Int {
	q := new(big.Int)
	rem := new(big.Int)
	q.QuoRem(a, b, rem)

	if rem.Sign() == 0 {
		return q
	}

	switch mode {
	case 0: // GMP_ROUND_ZERO: truncate toward zero (already done by Quo)
		// nothing to do
	case 1: // GMP_ROUND_PLUSINF: round toward +infinity
		// If result is positive and there's a remainder, round up
		if q.Sign() >= 0 {
			q.Add(q, big.NewInt(1))
		}
	case 2: // GMP_ROUND_MINUSINF: round toward -infinity
		// If result is negative and there's a remainder, round down
		if q.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		}
	}

	return q
}

// remWithRound computes remainder consistent with divWithRound.
func remWithRound(a, b *big.Int, mode int) *big.Int {
	q := divWithRound(a, b, mode)
	// rem = a - q*b
	rem := new(big.Int).Mul(q, b)
	rem.Sub(a, rem)
	return rem
}

// validateRoundMode validates the rounding mode parameter.
func validateRoundMode(ctx phpv.Context, funcName string, mode phpv.ZInt) error {
	if mode != 0 && mode != 1 && mode != 2 {
		return phpobj.ThrowError(ctx, phpobj.ValueError,
			funcName+"(): Argument #3 ($rounding_mode) must be one of GMP_ROUND_ZERO, GMP_ROUND_PLUSINF, or GMP_ROUND_MINUSINF")
	}
	return nil
}

// > func GMP gmp_div_q ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivQ(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal
	var round *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &b, &round)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_div_q", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_div_q", 2, "num2")
	if err != nil {
		return nil, err
	}

	mode := phpv.ZInt(0)
	if round != nil {
		mode = *round
		if err := validateRoundMode(ctx, "gmp_div_q", mode); err != nil {
			return nil, err
		}
	}

	if ib.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_q(): Argument #2 ($num2) Division by zero")
	}

	r := divWithRound(ia, ib, int(mode))

	return returnInt(ctx, r)
}

// > func GMP gmp_div_r ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivR(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal
	var round *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &b, &round)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_div_r", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_div_r", 2, "num2")
	if err != nil {
		return nil, err
	}

	mode := phpv.ZInt(0)
	if round != nil {
		mode = *round
		if err := validateRoundMode(ctx, "gmp_div_r", mode); err != nil {
			return nil, err
		}
	}

	if ib.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_r(): Argument #2 ($num2) Division by zero")
	}

	r := remWithRound(ia, ib, int(mode))

	return returnInt(ctx, r)
}

// > func GMP gmp_mod ( GMP $a , GMP $b )
func gmpMod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal

	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_mod", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_mod", 2, "num2")
	if err != nil {
		return nil, err
	}

	if ib.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_mod(): Argument #2 ($num2) Modulo by zero")
	}

	r := &big.Int{}
	r.Mod(ia, ib)

	return returnInt(ctx, r)
}

// gmp_div is an alias for gmp_div_q but with its own error message
func gmpDiv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal
	var round *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &b, &round)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_div", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_div", 2, "num2")
	if err != nil {
		return nil, err
	}

	mode := phpv.ZInt(0)
	if round != nil {
		mode = *round
		if err := validateRoundMode(ctx, "gmp_div", mode); err != nil {
			return nil, err
		}
	}

	if ib.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div(): Argument #2 ($num2) Division by zero")
	}

	r := divWithRound(ia, ib, int(mode))

	return returnInt(ctx, r)
}

// > func array gmp_div_qr ( GMP $a , GMP $b [, int $round = GMP_ROUND_ZERO ] )
func gmpDivQR(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal
	var round *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &b, &round)
	if err != nil {
		return nil, err
	}

	ia, err := readIntArg(ctx, a, "gmp_div_qr", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_div_qr", 2, "num2")
	if err != nil {
		return nil, err
	}

	mode := phpv.ZInt(0)
	if round != nil {
		mode = *round
		if err := validateRoundMode(ctx, "gmp_div_qr", mode); err != nil {
			return nil, err
		}
	}

	if ib.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "gmp_div_qr(): Argument #2 ($num2) Division by zero")
	}

	q := divWithRound(ia, ib, int(mode))
	r := remWithRound(ia, ib, int(mode))

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

	ia, err := readIntArg(ctx, a, "gmp_divexact", 1, "num1")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_divexact", 2, "num2")
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

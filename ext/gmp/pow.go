package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_pow ( GMP $base , int $exp )
func gmpPow(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var base, expVal *phpv.ZVal

	_, err := core.Expand(ctx, args, &base, &expVal)
	if err != nil {
		return nil, err
	}

	ibase, err := readIntArg(ctx, base, "gmp_pow", 1, "num")
	if err != nil {
		return nil, err
	}

	// Validate exponent type: must be int (PHP 8 rejects arrays/objects)
	if expVal == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"gmp_pow(): Argument #2 ($exponent) must be of type int, null given")
	}
	switch expVal.GetType() {
	case phpv.ZtInt:
		// ok
	case phpv.ZtFloat:
		// float is allowed with deprecation for whole numbers
	case phpv.ZtArray:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"gmp_pow(): Argument #2 ($exponent) must be of type int, array given")
	case phpv.ZtObject:
		obj, ok := expVal.Value().(*phpobj.ZObject)
		typeName := "object"
		if ok {
			typeName = string(obj.Class.GetName())
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"gmp_pow(): Argument #2 ($exponent) must be of type int, "+typeName+" given")
	case phpv.ZtBool:
		bname := "false"
		if expVal.Value().(phpv.ZBool) {
			bname = "true"
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"gmp_pow(): Argument #2 ($exponent) must be of type int, "+bname+" given")
	}

	expZ, err := expVal.As(ctx, phpv.ZtInt)
	if err != nil {
		return nil, err
	}
	exp := expZ.Value().(phpv.ZInt)

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

	ibase, err := readIntArg(ctx, base, "gmp_powm", 1, "num")
	if err != nil {
		return nil, err
	}
	iexp, err := readIntArg(ctx, exp, "gmp_powm", 2, "exponent")
	if err != nil {
		return nil, err
	}
	imod, err := readIntArg(ctx, mod, "gmp_powm", 3, "modulus")
	if err != nil {
		return nil, err
	}

	if imod.Sign() == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Modulo by zero")
	}

	// Negative exponents are not supported
	if iexp.Sign() < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_powm(): Argument #2 ($exponent) must be greater than or equal to 0")
	}

	r := &big.Int{}
	r.Exp(ibase, iexp, imod)

	return returnInt(ctx, r)
}

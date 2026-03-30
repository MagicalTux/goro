package gmp

import (
	"fmt"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// validateGMPArg checks that v is a GMP object and returns the object.
// funcName and argNum/argName are used for the PHP 8 error message.
func validateGMPArg(ctx phpv.Context, v *phpv.ZVal, funcName string, argNum int, argName string) (*phpobj.ZObject, error) {
	if v == nil || v.GetType() != phpv.ZtObject {
		typeName := "null"
		if v != nil {
			typeName = v.GetType().String()
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Argument #%d ($%s) must be of type GMP, %s given", funcName, argNum, argName, typeName))
	}
	obj, ok := v.Value().(*phpobj.ZObject)
	if !ok || obj.Class != GMP {
		typeName := v.GetType().String()
		if ok {
			typeName = string(obj.Class.GetName())
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Argument #%d ($%s) must be of type GMP, %s given", funcName, argNum, argName, typeName))
	}
	return obj, nil
}

// > func void gmp_setbit ( GMP $a , int $index [, bool $bit_on = TRUE ] )
func gmpSetbit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var numArg *phpv.ZVal
	var index phpv.ZInt
	var bitOn *phpv.ZBool

	_, err := core.Expand(ctx, args, &numArg, &index, &bitOn)
	if err != nil {
		return nil, err
	}

	a, err := validateGMPArg(ctx, numArg, "gmp_setbit", 1, "num")
	if err != nil {
		return nil, err
	}

	// PHP's GMP limits bit indices to GMP_MAX_BITCOUNT = INT_MAX * GMP_NUMB_BITS.
	// On 64-bit systems: 2147483647 * 64 = 137438953408.
	// This matches PHP behavior: values up to 0x3FFFFFFFF (17179869183) pass,
	// while 0x3FFFFFFFFF (274877906943) triggers the error.
	const (
		intMax      = phpv.ZInt(2147483647) // INT_MAX (32-bit C int)
		gmpNumbBits = phpv.ZInt(64)         // GMP_NUMB_BITS on 64-bit systems
		maxBitIndex = intMax * gmpNumbBits  // 137438953408
	)
	if index < 0 || index > maxBitIndex {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_setbit(): Argument #2 ($index) must be between 0 and %d * %d", intMax, gmpNumbBits))
	}

	opaque := a.GetOpaque(GMP)
	var i *big.Int
	if opaque != nil {
		i = opaque.(*big.Int)
	} else {
		i = &big.Int{}
	}

	b := uint(1)
	if bitOn != nil && !*bitOn {
		b = 0
	}

	r := new(big.Int).Set(i) // Copy first to avoid issues
	r.SetBit(r, int(index), b)

	a.SetOpaque(GMP, r)

	return nil, nil
}

// > func void gmp_clrbit ( GMP $a , int $index )
func gmpClrbit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var numArg *phpv.ZVal
	var index phpv.ZInt

	_, err := core.Expand(ctx, args, &numArg, &index)
	if err != nil {
		return nil, err
	}

	a, err := validateGMPArg(ctx, numArg, "gmp_clrbit", 1, "num")
	if err != nil {
		return nil, err
	}

	// PHP's GMP limits bit indices to GMP_MAX_BITCOUNT = INT_MAX * GMP_NUMB_BITS.
	const maxBitIndexClr = phpv.ZInt(2147483647) * phpv.ZInt(64) // 137438953408
	if index < 0 || index > maxBitIndexClr {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_clrbit(): Argument #2 ($index) must be between 0 and %d * %d", phpv.ZInt(2147483647), phpv.ZInt(64)))
	}

	opaque := a.GetOpaque(GMP)
	var i *big.Int
	if opaque != nil {
		i = opaque.(*big.Int)
	} else {
		i = &big.Int{}
	}

	r := new(big.Int).Set(i) // Copy first to avoid issues
	r.SetBit(r, int(index), 0)

	a.SetOpaque(GMP, r)

	return nil, nil
}

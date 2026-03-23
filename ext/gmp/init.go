package gmp

import (
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_init ( mixed $number [, int $base = 0 ] )
func gmpInit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num *phpv.ZVal
	var base *phpv.ZInt

	_, err := core.Expand(ctx, args, &num, &base)
	if err != nil {
		return nil, err
	}

	// Check if num is a GMP object - PHP disallows this
	if num.GetType() == phpv.ZtObject {
		if obj, ok := num.Value().(*phpobj.ZObject); ok && obj.Class == GMP {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "gmp_init(): Argument #1 ($num) must be of type string|int, GMP given")
		}
	}

	// Validate base
	if base != nil {
		b := int(*base)
		if b != 0 && (b < 2 || b > 62) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #2 ($base) must be between 2 and 62, or 0")
		}
	}

	var i *big.Int

	switch num.GetType() {
	case phpv.ZtNull, phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat:
		num, err = num.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		i = big.NewInt(int64(num.Value().(phpv.ZInt)))
	default:
		num, err = num.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		s := string(num.AsString(ctx))
		s = strings.TrimSpace(s)
		i = &big.Int{}
		b := 0
		if base != nil {
			b = int(*base)
		}
		if s == "" {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #1 ($num) is not an integer string")
		}
		_, ok := i.SetString(s, b)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #1 ($num) is not an integer string")
		}
	}

	return returnInt(ctx, i)
}

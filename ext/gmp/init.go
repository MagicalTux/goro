package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
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
		i = &big.Int{}
		if base == nil {
			_, ok := i.SetString(string(num.AsString(ctx)), 0)
			if !ok {
				return nil, errors.New("failed to parse integer")
			}
		} else {
			_, ok := i.SetString(string(num.AsString(ctx)), int(*base))
			if !ok {
				return nil, errors.New("failed to parse integer")
			}
		}
	}

	return returnInt(ctx, i)
}

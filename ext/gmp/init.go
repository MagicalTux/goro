package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

//> func GMP gmp_init ( mixed $number [, int $base = 0 ] )
func gmpInit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var num *core.ZVal
	var base *core.ZInt

	_, err := core.Expand(ctx, args, &num, &base)
	if err != nil {
		return nil, err
	}

	var i *big.Int

	switch num.GetType() {
	case core.ZtNull, core.ZtBool, core.ZtInt, core.ZtFloat:
		num, err = num.As(ctx, core.ZtInt)
		if err != nil {
			return nil, err
		}
		i = big.NewInt(int64(num.Value().(core.ZInt)))
	default:
		num, err = num.As(ctx, core.ZtString)
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

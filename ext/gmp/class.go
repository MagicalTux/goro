package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class GMP
var GMP = &phpobj.ZClass{
	Name: "GMP",
}

func init() {
	GMP.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				var num *phpv.ZVal
				var base *phpv.ZInt

				_, err := core.Expand(ctx, args, &num, &base)
				if err != nil {
					return nil, err
				}

				if num == nil {
					// No arguments: zero
					o.SetOpaque(GMP, big.NewInt(0))
					return nil, nil
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
					b := 0
					if base != nil {
						b = int(*base)
					}
					_, ok := i.SetString(string(num.AsString(ctx)), b)
					if !ok {
						return nil, errors.New("GMP::__construct(): Failed to parse number string")
					}
				}

				o.SetOpaque(GMP, i)
				return nil, nil
			}),
		},
		"__tostring": {
			Name: "__toString",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(GMP)
				if opaque == nil {
					return phpv.ZString("0").ZVal(), nil
				}
				i := opaque.(*big.Int)
				return phpv.ZString(i.String()).ZVal(), nil
			}),
		},
	}
}

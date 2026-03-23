package gmp

import (
	"errors"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func readInt(ctx phpv.Context, v *phpv.ZVal) (*big.Int, error) {
	var i *big.Int
	var err error

	switch v.GetType() {
	case phpv.ZtNull, phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat:
		v, err = v.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		i = big.NewInt(int64(v.Value().(phpv.ZInt)))
		return i, nil
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			// this is a gmp object
			return getGMPInt(obj), nil
		}
		fallthrough
	default:
		v, err = v.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		s := string(v.AsString(ctx))
		s = strings.TrimSpace(s)
		i = &big.Int{}
		_, ok := i.SetString(s, 0)
		if !ok {
			return nil, errors.New("Unable to convert variable to GMP - string is not an integer")
		}
		return i, nil
	}
}

func writeInt(ctx phpv.Context, v *phpv.ZVal, i *big.Int) error {
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			obj.SetOpaque(GMP, i)
			return nil
		}
	}
	return errors.New("expected parameter to be GMP")
}

func returnInt(ctx phpv.Context, i *big.Int) (*phpv.ZVal, error) {
	z, err := phpobj.NewZObjectOpaque(ctx, GMP, i)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

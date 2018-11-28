package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
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
		obj, ok := v.Value().(*core.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			// this is a gmp object
			i = obj.GetOpaque(GMP).(*big.Int)
			return i, nil
		}
		fallthrough
	default:
		v, err = v.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		i = &big.Int{}
		_, ok := i.SetString(string(v.AsString(ctx)), 0)
		if !ok {
			return nil, errors.New("Unable to convert variable to GMP - string is not an integer")
		}
		return i, nil
	}
}

func writeInt(ctx phpv.Context, v *phpv.ZVal, i *big.Int) error {
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*core.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			obj.SetOpaque(GMP, i)
			return nil
		}
	}
	return errors.New("expected parameter to be GMP")
}

func returnInt(ctx phpv.Context, i *big.Int) (*phpv.ZVal, error) {
	z, err := core.NewZObjectOpaque(ctx, GMP, i)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

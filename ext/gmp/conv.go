package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

func readInt(ctx core.Context, v *core.ZVal) (*big.Int, error) {
	var i *big.Int
	var err error

	switch v.GetType() {
	case core.ZtNull, core.ZtBool, core.ZtInt, core.ZtFloat:
		v, err = v.As(ctx, core.ZtInt)
		if err != nil {
			return nil, err
		}
		i = big.NewInt(int64(v.Value().(core.ZInt)))
		return i, nil
	case core.ZtObject:
		obj, ok := v.Value().(*core.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			// this is a gmp object
			i = obj.GetOpaque(GMP).(*big.Int)
			return i, nil
		}
		fallthrough
	default:
		v, err = v.As(ctx, core.ZtString)
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

func writeInt(ctx core.Context, v *core.ZVal, i *big.Int) error {
	switch v.GetType() {
	case core.ZtObject:
		obj, ok := v.Value().(*core.ZObject)
		if ok && obj.Class == GMP { // TODO check via instanceof (to be created)
			obj.SetOpaque(GMP, i)
			return nil
		}
	}
	return errors.New("expected parameter to be GMP")
}

func returnInt(ctx core.Context, i *big.Int) (*core.ZVal, error) {
	z, err := core.NewZObjectOpaque(ctx, GMP, i)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

package gmp

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// readInt converts a ZVal to *big.Int for GMP operations.
// It uses the calling function name from ctx.FuncName() for error messages.
func readInt(ctx phpv.Context, v *phpv.ZVal) (*big.Int, error) {
	var i *big.Int
	var err error

	switch v.GetType() {
	case phpv.ZtInt:
		return big.NewInt(int64(v.Value().(phpv.ZInt))), nil
	case phpv.ZtNull:
		// PHP GMP functions accept null as 0 for compat, but newer PHP throws TypeError
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, null given"))
	case phpv.ZtBool:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, bool given"))
	case phpv.ZtFloat:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, float given"))
	case phpv.ZtArray:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, array given"))
	case phpv.ZtResource:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, resource given"))
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP {
			return getGMPInt(obj), nil
		}
		if ok {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Number must be of type GMP|string|int, %s given", obj.Class.GetName()))
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, object given"))
	default:
		v, err = v.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		s := string(v.AsString(ctx))
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
		}
		i = &big.Int{}
		_, ok := i.SetString(s, 0)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
		}
		return i, nil
	}
}

func writeInt(ctx phpv.Context, v *phpv.ZVal, i *big.Int) error {
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP {
			obj.SetOpaque(GMP, i)
			return nil
		}
	}
	return fmt.Errorf("expected parameter to be GMP")
}

func returnInt(ctx phpv.Context, i *big.Int) (*phpv.ZVal, error) {
	z, err := phpobj.NewZObjectOpaque(ctx, GMP, i)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

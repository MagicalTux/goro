package gmp

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// readInt converts a ZVal to *big.Int for GMP operations.
// Uses generic error messages (without function/arg names).
// Prefer readIntArg for functions with specific error messages.
func readInt(ctx phpv.Context, v *phpv.ZVal) (*big.Int, error) {
	return readIntArg(ctx, v, "", 0, "")
}

// readIntArg converts a ZVal to *big.Int for GMP operations.
// funcName, argNum, argName are used for PHP-compatible error messages.
// e.g. funcName="gmp_abs", argNum=1, argName="num" → "gmp_abs(): Argument #1 ($num) ..."
// If funcName is empty, generic messages are used.
func readIntArg(ctx phpv.Context, v *phpv.ZVal, funcName string, argNum int, argName string) (*big.Int, error) {
	prefix := ""
	if funcName != "" {
		prefix = fmt.Sprintf("%s(): Argument #%d ($%s) ", funcName, argNum, argName)
	}

	switch v.GetType() {
	case phpv.ZtInt:
		return big.NewInt(int64(v.Value().(phpv.ZInt))), nil
	case phpv.ZtNull:
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, null given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Number must be of type GMP|string|int, null given")
	case phpv.ZtBool:
		bval := v.Value().(phpv.ZBool)
		bname := "false"
		if bval {
			bname = "true"
		}
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, "+bname+" given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Number must be of type GMP|string|int, bool given")
	case phpv.ZtFloat:
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, float given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Number must be of type GMP|string|int, float given")
	case phpv.ZtArray:
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, array given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Number must be of type GMP|string|int, array given")
	case phpv.ZtResource:
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, resource given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Number must be of type GMP|string|int, resource given")
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP {
			return getGMPInt(obj), nil
		}
		className := "object"
		if ok {
			className = string(obj.Class.GetName())
		}
		if prefix != "" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				prefix+"must be of type GMP|string|int, "+className+" given")
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, %s given", className))
	default:
		var err error
		v, err = v.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		s := string(v.AsString(ctx))
		s = strings.TrimSpace(s)
		if s == "" {
			if prefix != "" {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, prefix+"is not an integer string")
			}
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
		}
		// PHP does not accept leading '+' sign in GMP number strings
		if strings.HasPrefix(s, "+") {
			if prefix != "" {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, prefix+"is not an integer string")
			}
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
		}
		i := &big.Int{}
		_, ok := i.SetString(s, 0)
		if !ok {
			if prefix != "" {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, prefix+"is not an integer string")
			}
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

	// Register as a temporary object so its ID can be recycled at the next
	// statement boundary if it was not stored in a PHP variable (refcount == 0).
	id := z.ID
	ctx.Global().RegisterTempObject(id, func() bool {
		return z.RefCount() <= 0
	})

	return z.ZVal(), nil
}

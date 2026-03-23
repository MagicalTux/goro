package gmp

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// > class GMP
var GMP = &phpobj.ZClass{
	Name: "GMP",
	Attr: phpv.ZClassFinal,
}

// getGMPInt extracts the *big.Int from a GMP object.
func getGMPInt(o phpv.ZObject) *big.Int {
	opaque := o.GetOpaque(GMP)
	if opaque == nil {
		return big.NewInt(0)
	}
	return opaque.(*big.Int)
}

// readOperand converts a ZVal to *big.Int for operator overloading.
// Supports GMP objects, integers, and integer strings.
func readOperand(ctx phpv.Context, v *phpv.ZVal) (*big.Int, error) {
	if v == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, null given")
	}
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP {
			return getGMPInt(obj), nil
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Number must be of type GMP|string|int, %s given", obj.Class.GetName()))
	case phpv.ZtInt:
		return big.NewInt(int64(v.Value().(phpv.ZInt))), nil
	case phpv.ZtFloat:
		f := float64(v.Value().(phpv.ZFloat))
		// Check if float has fractional part
		intVal := int64(f)
		if float64(intVal) != f {
			// Deprecated: implicit conversion loses precision
			ctx.Deprecated("Implicit conversion from float %s to int loses precision", phpv.FormatFloatPrecision(f, -1))
		}
		return big.NewInt(intVal), nil
	case phpv.ZtString:
		s := string(v.Value().(phpv.ZString))
		s = strings.TrimSpace(s)
		i := &big.Int{}
		_, ok := i.SetString(s, 0)
		if !ok {
			// Try base 10 as fallback
			_, ok = i.SetString(s, 10)
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
			}
		}
		return i, nil
	case phpv.ZtNull:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, null given")
	case phpv.ZtBool:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, bool given")
	case phpv.ZtArray:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, array given")
	case phpv.ZtResource:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, resource given")
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Number must be of type GMP|string|int, %s given", v.GetType()))
	}
}

func init() {
	GMP.H = &phpv.ZClassHandlers{
		HandleCast:        gmpHandleCast,
		HandleDoOperation: gmpHandleDoOperation,
		HandleCompare:     gmpHandleCompare,
	}

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

				// Validate base
				if base != nil {
					b := int(*base)
					if b != 0 && (b < 2 || b > 62) {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #2 ($base) must be 0 or between 2 and 62")
					}
				}

				if num == nil {
					// No arguments: zero
					o.SetOpaque(GMP, big.NewInt(0))
					return nil, nil
				}

				// Check if num is a GMP object - PHP disallows this
				if num.GetType() == phpv.ZtObject {
					if obj, ok := num.Value().(*phpobj.ZObject); ok && obj.Class == GMP {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "gmp_init(): Argument #1 ($num) must be of type string|int, GMP given")
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
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #1 ($num) is not an integer string")
					}
					_, ok := i.SetString(s, b)
					if !ok {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #1 ($num) is not an integer string")
					}
				}

				o.SetOpaque(GMP, i)
				return nil, nil
			}),
		},
		"__tostring": {
			Name: "__toString",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				return phpv.ZString(i.String()).ZVal(), nil
			}),
		},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				arr := phpv.NewZArray()
				arr.OffsetSet(ctx, phpv.ZString("num").ZVal(), phpv.ZString(i.String()).ZVal())
				return arr.ZVal(), nil
			}),
		},
		"__serialize": {
			Name:      "__serialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				arr := phpv.NewZArray()
				// GMP serializes the number as hex string at index 0
				hexStr := fmt.Sprintf("%x", i)
				if i.Sign() < 0 {
					hexStr = fmt.Sprintf("-%x", new(big.Int).Abs(i))
				}
				arr.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZString(hexStr).ZVal())

				// Also serialize any dynamic properties at index 1 as sub-array
				dynProps := phpv.NewZArray()
				hasDynProps := false
				for prop := range o.IterProps(ctx) {
					if prop.VarName == "" {
						continue
					}
					v := o.GetPropValue(prop)
					dynProps.OffsetSet(ctx, prop.VarName.ZVal(), v)
					hasDynProps = true
				}
				// Also check for dynamic properties in hash table
				ht := o.HashTable()
				it := ht.NewIterator()
				for it.Valid(ctx) {
					k, _ := it.Key(ctx)
					v, _ := it.Current(ctx)
					if k.Value().GetType() == phpv.ZtString {
						dynProps.OffsetSet(ctx, k.Value(), v)
						hasDynProps = true
					}
					it.Next(ctx)
				}
				if hasDynProps {
					arr.OffsetSet(ctx, phpv.ZInt(1).ZVal(), dynProps.ZVal())
				}

				return arr.ZVal(), nil
			}),
		},
		"__unserialize": {
			Name:      "__unserialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				var data *phpv.ZVal
				_, err := core.Expand(ctx, args, &data)
				if err != nil {
					return nil, err
				}
				if data.GetType() != phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				arr := data.AsArray(ctx)

				// Index 0 should be the hex string
				numVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
				if numVal == nil || numVal.GetType() != phpv.ZtString {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				hexStr := string(numVal.AsString(ctx))
				if hexStr == "" {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				i := &big.Int{}
				_, ok := i.SetString(hexStr, 16)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				o.SetOpaque(GMP, i)

				// Index 1 should be dynamic properties (optional)
				propsVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
				if propsVal != nil && propsVal.GetType() == phpv.ZtArray {
					propsArr := propsVal.AsArray(ctx)
					for k, v := range propsArr.Iterate(ctx) {
						if k.GetType() == phpv.ZtString {
							o.ObjectSet(ctx, k.Value(), v)
						}
					}
				} else if propsVal != nil && !propsVal.IsNull() {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}

				return nil, nil
			}),
		},
	}
}

// gmpHandleCast handles (int), (float), (bool) casts for GMP objects.
func gmpHandleCast(ctx phpv.Context, o phpv.ZObject, t phpv.ZType) (phpv.Val, error) {
	i := getGMPInt(o)
	switch t {
	case phpv.ZtInt:
		return phpv.ZInt(i.Int64()), nil
	case phpv.ZtFloat:
		f, _ := new(big.Float).SetInt(i).Float64()
		return phpv.ZFloat(f), nil
	case phpv.ZtBool:
		return phpv.ZBool(i.Sign() != 0), nil
	default:
		return nil, fmt.Errorf("unsupported cast to %s", t)
	}
}

// gmpHandleCompare handles comparison between GMP objects (and GMP vs scalars).
func gmpHandleCompare(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
	ia := getGMPInt(a)
	ib := getGMPInt(b)
	return ia.Cmp(ib), nil
}

// gmpHandleDoOperation handles arithmetic/bitwise operator overloading for GMP.
func gmpHandleDoOperation(ctx phpv.Context, op int, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	itemOp := tokenizer.ItemType(op)

	// Handle unary operators (a == nil)
	if a == nil {
		ib, err := readOperand(ctx, b)
		if err != nil {
			return nil, err
		}
		switch itemOp {
		case tokenizer.Rune('-'):
			r := new(big.Int).Neg(ib)
			return returnInt(ctx, r)
		case tokenizer.Rune('+'):
			r := new(big.Int).Set(ib)
			return returnInt(ctx, r)
		case tokenizer.Rune('~'):
			r := new(big.Int).Not(ib)
			return returnInt(ctx, r)
		default:
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: GMP %s GMP", itemOp.OpString()))
		}
	}

	ia, err := readOperand(ctx, a)
	if err != nil {
		return nil, err
	}
	ib, err := readOperand(ctx, b)
	if err != nil {
		return nil, err
	}

	r := new(big.Int)

	switch itemOp {
	case tokenizer.Rune('+'), tokenizer.T_PLUS_EQUAL:
		r.Add(ia, ib)
	case tokenizer.Rune('-'), tokenizer.T_MINUS_EQUAL:
		r.Sub(ia, ib)
	case tokenizer.Rune('*'), tokenizer.T_MUL_EQUAL:
		r.Mul(ia, ib)
	case tokenizer.Rune('/'), tokenizer.T_DIV_EQUAL:
		if ib.Sign() == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Division by zero")
		}
		r.Quo(ia, ib)
	case tokenizer.Rune('%'), tokenizer.T_MOD_EQUAL:
		if ib.Sign() == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Modulo by zero")
		}
		r.Rem(ia, ib)
	case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Negative exponent is not supported")
		}
		r.Exp(ia, ib, nil)
	case tokenizer.Rune('|'), tokenizer.T_OR_EQUAL:
		r.Or(ia, ib)
	case tokenizer.Rune('&'), tokenizer.T_AND_EQUAL:
		r.And(ia, ib)
	case tokenizer.Rune('^'), tokenizer.T_XOR_EQUAL:
		r.Xor(ia, ib)
	case tokenizer.T_SL, tokenizer.T_SL_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Shift must be greater than or equal to 0")
		}
		r.Lsh(ia, uint(ib.Int64()))
	case tokenizer.T_SR, tokenizer.T_SR_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Shift must be greater than or equal to 0")
		}
		r.Rsh(ia, uint(ib.Int64()))
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: GMP %s GMP", itemOp.OpString()))
	}

	return returnInt(ctx, r)
}

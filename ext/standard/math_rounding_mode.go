package standard

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// RoundingMode enum backing values (matching PHP 8.4)
const (
	RoundingModeHalfAwayFromZero phpv.ZInt = 1
	RoundingModeHalfTowardsZero  phpv.ZInt = 2
	RoundingModeHalfEven         phpv.ZInt = 3
	RoundingModeHalfOdd          phpv.ZInt = 4
	RoundingModeTowardsZero      phpv.ZInt = 5
	RoundingModeAwayFromZero     phpv.ZInt = 6
	RoundingModeNegativeInfinity phpv.ZInt = 7
	RoundingModePositiveInfinity phpv.ZInt = 8
)

// roundingModeEnumCaseInit is a Runnable that creates a RoundingMode enum case object
type roundingModeEnumCaseInit struct {
	caseName     phpv.ZString
	backingValue phpv.ZInt
	enumClass    *phpobj.ZClass
}

func (r *roundingModeEnumCaseInit) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "RoundingMode::%s", r.caseName)
	return err
}

func (r *roundingModeEnumCaseInit) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	obj := phpobj.NewZObjectEnum(ctx, r.enumClass)
	obj.HashTable().SetString("name", phpv.ZString(r.caseName).ZVal())
	obj.HashTable().SetString("value", r.backingValue.ZVal())
	return obj.ZVal(), nil
}

// resolveEnumConst resolves a CompileDelayed constant value if needed
func resolveEnumConst(ctx phpv.Context, zc *phpobj.ZClass, caseName phpv.ZString) (phpv.Val, error) {
	cc, exists := zc.Const[caseName]
	if !exists {
		return nil, fmt.Errorf("enum case %s not found", caseName)
	}
	val := cc.Value
	if cd, ok := val.(*phpv.CompileDelayed); ok {
		z, err := cd.Run(ctx)
		if err != nil {
			return nil, err
		}
		cc.Value = z.Value()
		return z.Value(), nil
	}
	return val, nil
}

func newRoundingModeEnum() *phpobj.ZClass {
	cases := []struct {
		name  phpv.ZString
		value phpv.ZInt
	}{
		{"HalfAwayFromZero", RoundingModeHalfAwayFromZero},
		{"HalfTowardsZero", RoundingModeHalfTowardsZero},
		{"HalfEven", RoundingModeHalfEven},
		{"HalfOdd", RoundingModeHalfOdd},
		{"TowardsZero", RoundingModeTowardsZero},
		{"AwayFromZero", RoundingModeAwayFromZero},
		{"NegativeInfinity", RoundingModeNegativeInfinity},
		{"PositiveInfinity", RoundingModePositiveInfinity},
	}

	cls := &phpobj.ZClass{
		Name:            "RoundingMode",
		Type:            phpv.ZClassTypeEnum,
		Attr:            phpv.ZClassAttr(phpv.ZClassFinal),
		EnumBackingType: phpv.ZtInt,
		Const:           make(map[phpv.ZString]*phpv.ZClassConst),
		Methods:         make(map[phpv.ZString]*phpv.ZClassMethod),
		ImplementsStr:   []phpv.ZString{"UnitEnum", "BackedEnum"},
		InternalOnly:    true,
	}

	enumCases := make([]phpv.ZString, 0, len(cases))
	for _, c := range cases {
		enumCases = append(enumCases, c.name)
		cls.Const[c.name] = &phpv.ZClassConst{
			Value:     &phpv.CompileDelayed{V: &roundingModeEnumCaseInit{caseName: c.name, backingValue: c.value, enumClass: cls}},
			Modifiers: phpv.ZAttrPublic,
		}
		cls.ConstOrder = append(cls.ConstOrder, c.name)
	}
	cls.EnumCases = enumCases

	// Add cases() method
	cls.Methods["cases"] = &phpv.ZClassMethod{
		Name:      "cases",
		Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
		Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
			result := phpv.NewZArray()
			for _, caseName := range cls.EnumCases {
				val, err := resolveEnumConst(ctx, cls, caseName)
				if err != nil {
					return nil, err
				}
				result.OffsetSet(ctx, nil, val.ZVal())
			}
			return result.ZVal(), nil
		}),
		Class: cls,
	}

	// Add from() method
	cls.Methods["from"] = &phpv.ZClassMethod{
		Name:      "from",
		Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
		Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("from() expects exactly 1 argument")
			}
			needle := args[0]
			if needle.GetType() != phpv.ZtInt {
				actualType := needle.GetType().TypeName()
				if needle.GetType() == phpv.ZtObject {
					if obj, ok := needle.Value().(phpv.ZObject); ok {
						actualType = string(obj.GetClass().GetName())
					}
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("RoundingMode::from(): Argument #1 ($value) must be of type int, %s given", actualType))
			}
			for _, caseName := range cls.EnumCases {
				val, err := resolveEnumConst(ctx, cls, caseName)
				if err != nil {
					return nil, err
				}
				obj, ok := val.(*phpobj.ZObject)
				if !ok {
					continue
				}
				backingVal := obj.HashTable().GetString("value")
				if backingVal == nil {
					continue
				}
				eq, err := phpv.StrictEquals(ctx, needle, backingVal)
				if err != nil {
					return nil, err
				}
				if eq {
					return val.ZVal(), nil
				}
			}
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("%s is not a valid backing value for enum RoundingMode", needle.String()))
		}),
		Class: cls,
	}

	// Add tryFrom() method
	cls.Methods["tryfrom"] = &phpv.ZClassMethod{
		Name:      "tryFrom",
		Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
		Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("tryFrom() expects exactly 1 argument")
			}
			needle := args[0]
			if needle.GetType() != phpv.ZtInt {
				actualType := needle.GetType().TypeName()
				if needle.GetType() == phpv.ZtObject {
					if obj, ok := needle.Value().(phpv.ZObject); ok {
						actualType = string(obj.GetClass().GetName())
					}
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("RoundingMode::tryFrom(): Argument #1 ($value) must be of type int, %s given", actualType))
			}
			for _, caseName := range cls.EnumCases {
				val, err := resolveEnumConst(ctx, cls, caseName)
				if err != nil {
					return nil, err
				}
				obj, ok := val.(*phpobj.ZObject)
				if !ok {
					continue
				}
				backingVal := obj.HashTable().GetString("value")
				if backingVal == nil {
					continue
				}
				eq, err := phpv.StrictEquals(ctx, needle, backingVal)
				if err != nil {
					return nil, err
				}
				if eq {
					return val.ZVal(), nil
				}
			}
			return phpv.ZNULL.ZVal(), nil
		}),
		Class: cls,
	}

	return cls
}

// RoundingModeEnum is the built-in PHP 8.4 RoundingMode enum
var RoundingModeEnum = newRoundingModeEnum()

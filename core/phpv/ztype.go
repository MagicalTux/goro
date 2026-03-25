package phpv

import (
	"math"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
)

type ZType int

const (
	ZtNull ZType = iota
	ZtBool
	ZtInt
	ZtFloat
	ZtString
	ZtArray
	ZtObject
	ZtResource
	ZtCallable
	ZtMixed // PHP 8.0 mixed type (accepts any value)
	ZtVoid  // PHP 7.1 void return type
	ZtNever // PHP 8.1 never return type
)

// global NULL for easy call
var ZNULL = ZNull{}

var ZFalse = ZBool(false)
var ZTrue = ZBool(true)

// scalar stuff
type ZNull struct{}
type ZBool bool
type ZInt int64
type ZFloat float64
type ZString string

func (z ZNull) GetType() ZType {
	return ZtNull
}

func (z ZNull) ZVal() *ZVal {
	return NewZVal(ZNull{})
}

func (z ZNull) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtNull:
		return ZNull{}, nil
	case ZtBool:
		return ZBool(false), nil
	case ZtInt:
		return ZInt(0), nil
	case ZtFloat:
		return ZFloat(0), nil
	case ZtString:
		return ZString(""), nil
	case ZtArray:
		return NewZArray(), nil
	case ZtObject:
		if NewStdClassFunc != nil {
			return NewStdClassFunc(ctx)
		}
	}
	return nil, nil
}

// scalarToObject converts a scalar value to a stdClass object with a "scalar" property.
// This is the PHP behavior for (object)$scalar.
func scalarToObject(ctx Context, v *ZVal) (Val, error) {
	if NewStdClassFunc == nil {
		return nil, nil
	}
	obj, err := NewStdClassFunc(ctx)
	if err != nil {
		return nil, err
	}
	obj.ObjectSet(ctx, ZString("scalar"), v)
	return obj, nil
}

func (z ZNull) Value() Val {
	return z
}

func (z ZBool) GetType() ZType {
	return ZtBool
}

func (z ZBool) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtNull:
		return ZNull{}, nil
	case ZtBool:
		return z, nil
	case ZtInt:
		if z {
			return ZInt(1), nil
		} else {
			return ZInt(0), nil
		}
	case ZtFloat:
		if z {
			return ZFloat(1), nil
		} else {
			return ZFloat(0), nil
		}
	case ZtString:
		if z {
			return ZString("1"), nil
		} else {
			return ZString(""), nil
		}
	case ZtArray:
		arr := NewZArray()
		arr.OffsetSet(ctx, nil, z.ZVal())
		return arr, nil
	case ZtObject:
		return scalarToObject(ctx, z.ZVal())
	}
	return nil, nil
}

func (z ZBool) ZVal() *ZVal {
	return NewZVal(z)
}

func (z ZBool) String() string {
	if z {
		return "1"
	} else {
		return ""
	}
}

func (z ZBool) Value() Val {
	return z
}

func (z ZInt) GetType() ZType {
	return ZtInt
}

func (z ZInt) ZVal() *ZVal {
	return NewZVal(z)
}

func (z ZInt) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtBool:
		return ZBool(z != 0), nil
	case ZtInt:
		return z, nil
	case ZtFloat:
		return ZFloat(z), nil
	case ZtString:
		return ZString(strconv.FormatInt(int64(z), 10)), nil
	case ZtArray:
		r := NewZArray()
		r.OffsetSet(ctx, nil, z.ZVal())
		return r, nil
	case ZtObject:
		return scalarToObject(ctx, z.ZVal())
	}
	return nil, nil
}

func (v ZInt) String() string {
	s := strconv.FormatInt(int64(v), 10)
	return s
}

func (v ZInt) Value() Val {
	return v
}

func (z ZFloat) GetType() ZType {
	return ZtFloat
}

func (z ZFloat) ZVal() *ZVal {
	return NewZVal(z)
}

func (z ZFloat) AsVal(ctx Context, t ZType) (Val, error) {
	f := float64(z)
	// PHP 8.5+: Warn when coercing NAN to any non-float type
	if math.IsNaN(f) && ctx != nil && t != ZtFloat {
		targetName := t.TypeName()
		if err := ctx.Warn("unexpected NAN value was coerced to %s", targetName, logopt.NoFuncName(true)); err != nil {
			// If the error handler modified the value or threw, still
			// perform the coercion but propagate the error.
			switch t {
			case ZtBool:
				return ZBool(true), err
			case ZtInt:
				return ZInt(0), err
			case ZtString:
				prec := 14
				prec = GetPrecision(ctx)
				return ZString(FormatFloatPrecision(f, prec)), err
			case ZtArray:
				arr := NewZArray()
				arr.OffsetSet(ctx, nil, z.ZVal())
				return arr, err
			case ZtObject:
				v, _ := scalarToObject(ctx, z.ZVal())
				return v, err
			case ZtNull:
				return ZNull{}, err
			}
			return nil, err
		}
	}
	switch t {
	case ZtBool:
		return ZBool(z != 0), nil
	case ZtInt:
		if math.IsNaN(f) || math.IsInf(f, 0) || f > math.MaxInt64 || f < math.MinInt64 {
			if ctx != nil && !math.IsNaN(f) {
				if err := ctx.Warn("The float %s is not representable as an int, cast occurred", FormatFloat(f), logopt.NoFuncName(true)); err != nil {
					return ZInt(0), err
				}
			}
			return ZInt(0), nil
		}
		return ZInt(z), nil
	case ZtFloat:
		return z, nil
	case ZtString:
		prec := 14
		if ctx != nil {
			prec = GetPrecision(ctx)
		}
		return ZString(FormatFloatPrecision(f, prec)), nil
	case ZtArray:
		arr := NewZArray()
		arr.OffsetSet(ctx, nil, z.ZVal())
		return arr, nil
	case ZtObject:
		return scalarToObject(ctx, z.ZVal())
	case ZtNull:
		return ZNull{}, nil
	}
	return nil, nil
}

func (v ZFloat) String() string {
	return FormatFloat(float64(v))
}

// FormatFloat formats a float64 value in PHP-compatible style for var_dump
// and similar output (serialize_precision=-1 behavior). It uses the shortest
// decimal representation that round-trips back to the same float64 value.
// PHP uses decimal form for values where -4 <= exponent < 17, and scientific
// notation otherwise, always ensuring a decimal point in E notation.
func FormatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	if math.IsNaN(f) {
		return "NAN"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}

	// Determine the exponent to decide format
	abs := math.Abs(f)
	exp := math.Floor(math.Log10(abs))

	if exp >= -4 && exp < 17 {
		// Use decimal notation
		return strconv.FormatFloat(f, 'f', -1, 64)
	}

	// Use scientific notation
	s := strconv.FormatFloat(f, 'E', -1, 64)
	return phpFormatSci(s)
}

// phpFormatSci adjusts Go's scientific notation to match PHP style:
// - ensures decimal point in mantissa (1E+20 → 1.0E+20)
// - strips leading zeros from exponent (E+07 → E+7)
func phpFormatSci(s string) string {
	idx := strings.Index(s, "E")
	if idx < 0 {
		return s
	}

	mantissa := s[:idx]
	expPart := s[idx:] // "E+07" or "E-04"

	// Ensure mantissa has a decimal point
	if !strings.Contains(mantissa, ".") {
		mantissa = mantissa + ".0"
	}

	// Strip leading zeros from exponent (keep at least one digit)
	if len(expPart) > 2 {
		sign := expPart[1:2] // "+" or "-"
		digits := expPart[2:]
		for len(digits) > 1 && digits[0] == '0' {
			digits = digits[1:]
		}
		expPart = "E" + sign + digits
	}

	return mantissa + expPart
}

// FormatFloatPrecision formats a float64 using the given precision (like PHP's
// precision ini setting). Used for echo/print and string casting.
// When prec < 0, uses PHP's "shortest unique representation" mode.
// When prec == 0, PHP uses 1 significant digit (minimum).
func FormatFloatPrecision(f float64, prec int) string {
	if prec < 0 {
		// PHP precision=-1: use shortest unique representation
		// (same as serialize_precision=-1 behavior)
		return FormatFloat(f)
	}

	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	if math.IsNaN(f) {
		return "NAN"
	}

	// PHP treats precision=0 as 1 significant digit (minimum)
	if prec == 0 {
		prec = 1
	}

	s := strconv.FormatFloat(f, 'G', prec, 64)

	// PHP formats scientific notation as e.g. "1.23E+7" not "1.23E+07"
	// Also ensures there's a decimal point before E: "1.0E+20" not "1E+20"
	return phpFormatSci(s)
}

// FormatFloatSerialize formats a float64 for serialize() when serialize_precision=0.
// PHP's zend_gcvt at ndigit=0 always uses scientific notation (1 significant digit
// in E format), matching the behavior where the sci threshold is decpt > 0.
func FormatFloatSerialize(f float64) string {
	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	if math.IsNaN(f) {
		return "NAN"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	// 'E' format with prec=0 gives 1 significant digit in scientific notation
	s := strconv.FormatFloat(f, 'E', 0, 64)
	return phpFormatSci(s)
}

// GetPrecision reads the 'precision' INI setting from the context.
// Returns 14 (PHP default) if not set or context is nil.
func GetPrecision(ctx Context) int {
	if ctx == nil {
		return 14
	}
	v := ctx.GetConfig("precision", ZInt(14).ZVal())
	if v == nil {
		return 14
	}
	return int(v.AsInt(ctx))
}

// GetSerializePrecision reads the 'serialize_precision' INI setting from the context.
// Returns -1 (PHP default) if not set or context is nil.
func GetSerializePrecision(ctx Context) int {
	if ctx == nil {
		return -1
	}
	v := ctx.GetConfig("serialize_precision", ZInt(-1).ZVal())
	if v == nil {
		return -1
	}
	return int(v.AsInt(ctx))
}

func (v ZFloat) Value() Val {
	return v
}

// FloatToIntImplicit converts a float to int for implicit conversion contexts (bitwise ops,
// array offsets, modulo, function args with int type, etc.). It emits the PHP 8.1+
// "Deprecated: Implicit conversion from float X to int loses precision" when the float
// has a non-zero fractional part.
func FloatToIntImplicit(ctx Context, f ZFloat) (ZInt, error) {
	fv := float64(f)
	if math.IsNaN(fv) || math.IsInf(fv, 0) || fv > math.MaxInt64 || fv < math.MinInt64 {
		if ctx != nil {
			if err := ctx.Warn("The float %s is not representable as an int, cast occurred", FormatFloat(fv), logopt.NoFuncName(true)); err != nil {
				return ZInt(0), err
			}
		}
		return ZInt(0), nil
	}
	// Check if the float has a fractional part
	if fv != math.Trunc(fv) {
		if ctx != nil {
			if err := ctx.Deprecated("Implicit conversion from float %s to int loses precision", FormatFloat(fv), logopt.NoFuncName(true)); err != nil {
				return ZInt(fv), err
			}
		}
	}
	return ZInt(fv), nil
}

func (zt ZType) String() string {
	switch zt {
	case ZtNull:
		return "NULL"
	case ZtBool:
		return "boolean"
	case ZtInt:
		return "integer"
	case ZtFloat:
		return "double"
	case ZtString:
		return "string"
	case ZtArray:
		return "array"
	case ZtObject:
		return "object"
	case ZtResource:
		return "resource"
	default:
		return "?"
	}
}

// TypeName returns the PHP 8-style type name (lowercase, short form)
func (zt ZType) TypeName() string {
	switch zt {
	case ZtNull:
		return "null"
	case ZtBool:
		return "bool"
	case ZtInt:
		return "int"
	case ZtFloat:
		return "float"
	case ZtString:
		return "string"
	case ZtArray:
		return "array"
	case ZtObject:
		return "object"
	case ZtResource:
		return "resource"
	case ZtMixed:
		return "mixed"
	case ZtVoid:
		return "void"
	case ZtNever:
		return "never"
	default:
		return "?"
	}
}

// ZValTypeName returns the PHP type name of a ZVal, including the class name
// for objects (e.g. "stdClass" instead of just "object").
func ZValTypeName(z *ZVal) string {
	if z == nil {
		return "null"
	}
	if z.GetType() == ZtObject {
		if obj, ok := z.Value().(ZObject); ok {
			return string(obj.GetClass().GetName())
		}
	}
	return z.GetType().TypeName()
}

// ZValTypeNameDetailed returns the PHP 8 type name for error messages, where
// booleans are "true" or "false" instead of "bool".
func ZValTypeNameDetailed(z *ZVal) string {
	if z == nil {
		return "null"
	}
	if z.GetType() == ZtBool {
		if z.Value().(ZBool) {
			return "true"
		}
		return "false"
	}
	if z.GetType() == ZtObject {
		if obj, ok := z.Value().(ZObject); ok {
			return string(obj.GetClass().GetName())
		}
	}
	return z.GetType().TypeName()
}

func IsNull(val Val) bool {
	return val == nil || val.GetType() == ZtNull
}

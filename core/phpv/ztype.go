package phpv

import (
	"math"
	"strconv"
	"strings"
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
		// TODO: cyclic dependency phpv -> phpobj
		// return phpobj.NewZObject(ctx, phpobj.StdClass)
	}
	return nil, nil
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
	switch t {
	case ZtBool:
		return ZBool(z != 0), nil
	case ZtInt:
		if math.IsNaN(float64(z)) || math.IsInf(float64(z), 0) || float64(z) > math.MaxInt64 || float64(z) < math.MinInt64 {
			if ctx != nil {
				ctx.Warn("The float %s is not representable as an int, cast occurred", FormatFloat(float64(z)))
			}
		}
		return ZInt(z), nil
	case ZtFloat:
		return z, nil
	case ZtString:
		return ZString(FormatFloatPrecision(float64(z), 14)), nil
	case ZtArray:
		arr := NewZArray()
		arr.OffsetSet(ctx, nil, z.ZVal())
		return arr, nil
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
func FormatFloatPrecision(f float64, prec int) string {
	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	if math.IsNaN(f) {
		return "NAN"
	}

	s := strconv.FormatFloat(f, 'G', prec, 64)

	// PHP formats scientific notation as e.g. "1.23E+7" not "1.23E+07"
	// Also ensures there's a decimal point before E: "1.0E+20" not "1E+20"
	eIndex := strings.Index(s, "E")
	if eIndex > 0 && eIndex < len(s)-1 {
		// Ensure decimal point exists in mantissa
		pre := s[:eIndex]
		if !strings.Contains(pre, ".") {
			pre = pre + ".0"
		}

		// Remove leading zero from exponent
		post := s[eIndex+2:] // skip "E+" or "E-"
		sign := s[eIndex+1 : eIndex+2]
		for len(post) > 1 && post[0] == '0' {
			post = post[1:]
		}
		s = pre + "E" + sign + post
	}
	return s
}

func (v ZFloat) Value() Val {
	return v
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
	default:
		return "?"
	}
}

func IsNull(val Val) bool {
	return val == nil || val.GetType() == ZtNull
}

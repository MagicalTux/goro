package phpv

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (z ZString) GetType() ZType {
	return ZtString
}

func (z ZString) ZVal() *ZVal {
	return NewZVal(z)
}

func (z ZString) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtBool:
		return ZBool(z != "" && z != "0"), nil
	case ZtInt:
		v, _ := z.AsNumeric()
		switch v := v.(type) {
		case ZInt:
			return v, nil
		case ZFloat:
			return ZInt(v), nil
		default:
			return nil, nil
		}
	case ZtFloat:
		v, _ := z.AsNumeric()
		switch v := v.(type) {
		case ZInt:
			return ZFloat(v), nil
		case ZFloat:
			return v, nil
		default:
			return nil, nil
		}
	case ZtString:
		return z, nil
	case ZtArray:
		arr := NewZArray()
		arr.OffsetSet(ctx, ZInt(0), z.ZVal())
		return arr, nil
	}
	return nil, nil
}

func (s ZString) ToLower() ZString {
	return ZString(strings.ToLower(string(s)))
}

func (s ZString) ToUpper() ZString {
	return ZString(strings.ToUpper(string(s)))
}

func (s ZString) LooksInt() bool {
	var first bool
	if len(s) == 0 {
		return false
	}
	first = true
	for _, c := range s {
		if first && (c == ' ' || c == '-') {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		first = false
	}
	return true
}

func (s ZString) IsNumeric() bool {
	var gotDot, gotE, first bool
	if len(s) == 0 {
		return false
	}
	first = true
	for _, c := range s {
		if first && unicode.IsSpace(c) {
			continue
		}
		if first && (c == '+' || c == '-') {
			// good
			first = false
			continue
		}
		if c == '.' && !gotDot && !gotE {
			gotDot = true
			first = false
			continue // good
		}
		if c == 'e' && !gotE {
			gotE = true
			first = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		first = false
	}
	return true
}

func (z ZString) ContainsInvalidNumeric() bool {
	// attempt to convert z to a numeric type. First, get rid of initial spaces
	var r rune
	var l int

	for {
		if len(z) < 1 {
			return false
		}
		r, l = utf8.DecodeRuneInString(string(z))

		if !unicode.IsSpace(r) {
			break
		}
		z = z[l:]
	}

	p := 0
	i := 0

	for ; i < len(z); i++ {
		c := z[i]
		if c >= '0' && c <= '9' {
			if p == 0 || p == 3 {
				p += 1
			}
			continue
		}
		if c == '+' || c == '-' {
			if p == 0 || p == 3 {
				p += 1
				continue
			}
			return true
		}
		if c == '.' {
			if p == 1 {
				p = 2
				continue
			}
			return true
		}
		if c == 'e' || c == 'E' {
			if p < 3 {
				p = 3
				continue
			}
			return true
		}
		return true
	}
	return false
}

func (z ZString) AsNumeric() (Val, error) {
	// attempt to convert z to a numeric type. First, get rid of initial spaces
	var r rune
	var l int

	for {
		if len(z) < 1 {
			return ZInt(0), errors.New("no numeric value")
		}
		r, l = utf8.DecodeRuneInString(string(z))

		if !unicode.IsSpace(r) {
			break
		}
		z = z[l:]
	}

	p := 0
	i := 0

	for ; i < len(z); i++ {
		c := z[i]
		if c >= '0' && c <= '9' {
			if p == 0 || p == 3 {
				p += 1
			}
			continue
		}
		if c == '+' || c == '-' {
			if p == 0 || p == 3 {
				p += 1
				continue
			}
			break
		}
		if c == '.' {
			if p == 1 {
				p = 2
				continue
			}
			break
		}
		if c == 'e' || c == 'E' {
			if p < 3 {
				p = 3
				continue
			}
			break
		}
		break
	}

	if p <= 1 {
		// integer value (NB: might be too large to fit in 64 bits, in which case we'll parse as float)
		v, err := strconv.ParseInt(string(z[:i]), 10, 64)
		if err == nil {
			return ZInt(v), nil
		}
	}

	v, err := strconv.ParseFloat(string(z[:i]), 64)
	if err == nil {
		return ZFloat(v), nil
	}

	return ZInt(0), err
}

func (v ZString) String() string {
	return string(v)
}

func ZStr(s string) *ZVal {
	return NewZVal(ZString(s))
}

func (v ZString) Value() Val {
	return v
}

func (z ZString) Array() ZStringArray {
	return ZStringArray{&z}
}

type ZStringArray struct {
	*ZString
}

func (z ZStringArray) String() ZString {
	return *z.ZString
}

func (z ZStringArray) OffsetGet(ctx Context, key Val) (*ZVal, error) {
	if key.GetType() != ZtInt {
		if err := ctx.Warn("Illegal string offset '%s'", key.String()); err != nil {
			return nil, err
		}
	}
	val, _ := key.AsVal(ctx, ZtInt)
	i := int(val.(ZInt))
	s := *z.ZString
	if i < 0 || i >= len(s) {
		return ZStr(""), ctx.Warn("Uninitialized string offset: %v", key.String())
	}
	c := ZString(s[i])

	return c.ZVal(), nil
}

func (z ZStringArray) OffsetSet(ctx Context, key Val, value *ZVal) error {
	var i int
	s := *z.ZString
	if key == nil {
		i = len(s)
	} else {
		if key.GetType() != ZtInt {
			return ctx.Warn("Illegal string offset '%s'", key.String())
		}
		val, _ := key.AsVal(ctx, ZtInt)
		i = int(val.(ZInt))
	}

	c := value.AsString(ctx)

	if i < 0 {
		i = len(s) + i
	} else if i >= len(s) {
		s = s + ZString(strings.Repeat(" ", i-len(s)+1))
	}

	if i >= 0 && i < len(s) {
		*z.ZString = s[0:i] + c + s[i+1:]
	}

	return nil
}

func (z ZStringArray) OffsetUnset(ctx Context, key Val) error {
	if key.GetType() != ZtInt {
		return ctx.Warn("Illegal string offset '%s'", key.String())
	}
	val, _ := key.AsVal(ctx, ZtInt)
	i := val.(ZInt)
	s := *z.ZString
	*z.ZString = s[0:i] + s[i+1:]
	return nil
}

func (z ZStringArray) OffsetExists(ctx Context, key Val) (bool, error) {
	val, _ := key.AsVal(ctx, ZtInt)
	i := int(val.(ZInt))
	return i >= 0 && i < len(*z.ZString), nil
}

func (z ZStringArray) OffsetCheck(ctx Context, key Val) (*ZVal, bool, error) {
	if key.GetType() != ZtInt {
		return nil, false, nil
	}
	val, _ := key.AsVal(ctx, ZtInt)
	i := int(val.(ZInt))
	if i < 0 && i >= len(*z.ZString) {
		return nil, false, nil
	}

	c := ZString((*z.ZString)[i])
	return c.ZVal(), true, nil
}

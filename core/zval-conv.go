package core

import (
	"fmt"
	"strconv"
)

func (z *ZVal) CastTo(ctx Context, t ZType) error {
	z2, err := z.As(ctx, t)
	if err != nil {
		return err
	}
	z.v = z2.v
	return nil
}

func (z *ZVal) As(ctx Context, t ZType) (*ZVal, error) {
	r, err := z.AsVal(ctx, t)
	return &ZVal{r}, err
}

func (z *ZVal) AsVal(ctx Context, t ZType) (Val, error) {
	if z.GetType() == t {
		// nothing to do
		return z, nil
	}
	if t == ZtNull {
		// cast to NULL can only result into null
		return &ZVal{ZNull{}}, nil
	}

	if z == nil || z.v == nil {
		v, err := ZNull{}.AsVal(ctx, t)
		if err != nil {
			return nil, err
		}
		return v.ZVal(), nil
	}

	v, err := z.v.AsVal(ctx, t)
	if err != nil {
		return nil, err
	}
	if v != nil {
		return &ZVal{v}, nil
	}

	return nil, fmt.Errorf("todo %s => %s", z.v.GetType(), t)
}

func (z *ZVal) AsNumeric(ctx Context) (*ZVal, error) {
	if z == nil {
		return &ZVal{nil}, nil
	}
	switch n := z.v.(type) {
	case ZInt:
		return z, nil
	case ZFloat:
		return z, nil
	case ZString:
		v1, err := strconv.ParseInt(string(n), 10, 64)
		if err == nil {
			return ZInt(v1).ZVal(), nil
		}
		// fallback to float
		return z.As(ctx, ZtFloat)
	default:
		return z.As(ctx, ZtInt)
	}
}

func (z *ZVal) IsNull() bool {
	if z == nil {
		return true
	}
	if z.v == nil {
		return true
	}
	return false
}

func (z *ZVal) AsBool(ctx Context) ZBool {
	// simple method for quick conversion
	r, err := z.As(ctx, ZtBool)
	if err != nil {
		return false
	}
	return r.v.(ZBool)
}

func (z *ZVal) AsInt(ctx Context) ZInt {
	// simple method for quick conversion
	r, err := z.As(ctx, ZtInt)
	if err != nil {
		return 0
	}
	return r.Value().(ZInt)
}

func (z *ZVal) AsFloat(ctx Context) ZFloat {
	r, err := z.As(ctx, ZtFloat)
	if err != nil {
		return 0
	}
	return r.Value().(ZFloat)
}

func (z *ZVal) AsString(ctx Context) ZString {
	r, err := z.As(ctx, ZtString)
	if err != nil {
		return ""
	}
	return r.Value().(ZString)
}

func (z *ZVal) String() string {
	//Typically, use z.As(ctx, ZtString)
	switch n := z.Value().(type) {
	case nil:
		return ""
	case ZBool:
		if n {
			return "1"
		} else {
			return ""
		}
	case ZInt:
		return strconv.FormatInt(int64(n), 10)
	case ZFloat:
		return strconv.FormatFloat(float64(n), 'G', -1, 64)
	case ZString:
		return string(n)
	}
	switch z.GetType() {
	case ZtNull:
		return ""
	case ZtArray:
		return "Array"
	case ZtObject:
		return "Object"
	case ZtResource:
		return "Resource"
	default:
		return fmt.Sprintf("Unknown[%T]", z.v)
	}
}

func (z *ZVal) Array() ZArrayAccess {
	if r, ok := z.v.(ZArrayAccess); ok {
		return r
	}
	return nil
}

func (z *ZVal) NewIterator() ZIterator {
	if r, ok := z.v.(ZIterable); ok {
		return r.NewIterator()
	}
	return nil
}

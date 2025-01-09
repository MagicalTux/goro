package phpv

import (
	"strconv"
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
		return ZInt(z), nil
	case ZtFloat:
		return z, nil
	case ZtString:
		precision := int(ctx.GetConfig("precision", ZInt(14).ZVal()).AsInt(ctx))
		return ZString(strconv.FormatFloat(float64(z), 'G', precision, 64)), nil
	case ZtArray:
		arr := NewZArray()
		arr.OffsetSet(ctx, nil, z.ZVal())
		return arr, nil
	}
	return nil, nil
}

func (v ZFloat) String() string {
	s := strconv.FormatFloat(float64(v), 'f', -1, 64)
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

func IsNull(val Val) bool {
	return val == nil || val.GetType() == ZtNull
}

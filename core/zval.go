package core

import (
	"fmt"
	"strconv"
)

type Val interface {
	GetType() ZType
	ZVal() *ZVal
	AsVal(ctx Context, t ZType) (Val, error)
}

type ZVal struct {
	v Val
}

type runZVal struct {
	v Val
	l *Loc
}

func (z *ZVal) GetType() ZType {
	if z == nil {
		return ZtNull
	}
	if z.v == nil {
		return ZtNull
	}
	return z.v.GetType()
}

func (z *runZVal) Run(ctx Context) (*ZVal, error) {
	return &ZVal{z.v}, nil
}

func (z *runZVal) Loc() *Loc {
	return z.l
}

func (z *ZVal) ZVal() *ZVal {
	return z
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
	return r.v.(ZInt)
}

func (z *ZVal) String() string {
	//Typically, use z.As(ctx, ZtString)
	switch n := z.v.(type) {
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

func (z *ZVal) Value() Val {
	if sz, ok := z.v.(*ZVal); ok {
		return sz.Value()
	}
	return z.v
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

package core

import "strconv"

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
)

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
	return &ZVal{nil}
}

func (z ZNull) As(ctx Context, t ZType) (Val, error) {
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
		// TODO ZtObject
	}
	return nil, nil
}

func (z ZBool) GetType() ZType {
	return ZtBool
}

func (z ZBool) As(ctx Context, t ZType) (Val, error) {
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
	}
	return nil, nil
}

func (z ZBool) ZVal() *ZVal {
	return &ZVal{z}
}

func (z ZString) GetType() ZType {
	return ZtString
}

func (z ZString) ZVal() *ZVal {
	return &ZVal{z}
}

func (z ZString) As(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtBool:
		return ZBool(z != "" && z != "0"), nil
	case ZtInt:
		i, _ := strconv.ParseInt(string(z), 10, 64)
		return ZInt(i), nil
	case ZtFloat:
		f, _ := strconv.ParseFloat(string(z), 64)
		return ZFloat(f), nil
	case ZtString:
		return z, nil
	}
	return nil, nil
}

func (z ZInt) GetType() ZType {
	return ZtInt
}

func (z ZInt) ZVal() *ZVal {
	return &ZVal{z}
}

func (z ZInt) As(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtBool:
		return ZBool(z != 0), nil
	case ZtInt:
		return z, nil
	case ZtFloat:
		return ZFloat(z), nil
	case ZtString:
		return ZString(strconv.FormatInt(int64(z), 10)), nil
	}
	return nil, nil
}

func (z ZFloat) GetType() ZType {
	return ZtFloat
}

func (z ZFloat) ZVal() *ZVal {
	return &ZVal{z}
}

func (z ZFloat) As(ctx Context, t ZType) (Val, error) {
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
	}
	return nil, nil
}

func ZStr(s string) *ZVal {
	return &ZVal{ZString(s)}
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

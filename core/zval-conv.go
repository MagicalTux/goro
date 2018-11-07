package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (z *ZVal) As(ctx Context, t ZType) (*ZVal, error) {
	if z.v.GetType() == t {
		// nothing to do
		return z, nil
	}

	switch t {
	case ZtNull:
		return &ZVal{nil}, nil
	case ZtInt:
		switch n := z.v.(type) {
		case ZFloat:
			return &ZVal{ZInt(n)}, nil
		case ZBool:
			if n {
				return &ZVal{ZInt(1)}, nil
			} else {
				return &ZVal{ZInt(0)}, nil
			}
		default:
			s, _ := z.As(ctx, ZtString)
			i, _ := strconv.ParseInt(string(s.v.(ZString)), 0, 64)
			return &ZVal{ZInt(i)}, nil
		}
	case ZtFloat:
		switch n := z.v.(type) {
		case ZInt:
			return &ZVal{ZFloat(n)}, nil
		case ZBool:
			if n {
				return &ZVal{ZFloat(1)}, nil
			} else {
				return &ZVal{ZFloat(0)}, nil
			}
		default:
			s, _ := z.As(ctx, ZtString)
			i, _ := strconv.ParseFloat(string(s.v.(ZString)), 64)
			return &ZVal{ZFloat(i)}, nil
		}
	case ZtString:
		switch n := z.v.(type) {
		case nil:
			return &ZVal{ZString("")}, nil
		case ZBool:
			if n {
				return &ZVal{ZString("1")}, nil
			} else {
				return &ZVal{ZString("")}, nil
			}
		case ZInt:
			return &ZVal{ZString(strconv.FormatInt(int64(n), 10))}, nil
		case ZFloat:
			return &ZVal{ZString(strconv.FormatFloat(float64(n), 'g', -1, 64))}, nil
		case ZString:
			return &ZVal{ZString(string(n))}, nil
		}
		switch z.GetType() {
		case ZtNull:
			return &ZVal{ZString("")}, nil
		case ZtArray:
			return &ZVal{ZString("Array")}, nil
		case ZtObject:
			// TODO call __toString()
			return &ZVal{ZString("")}, nil // fatal error if no __toString() method
		case ZtResource:
			return &ZVal{ZString("Resource id #")}, nil // TODO
		default:
			return &ZVal{ZString(fmt.Sprintf("Unknown[%T]", z.v))}, nil
		}
	}

	return nil, errors.New("todo")
}

func (z *ZVal) AsNumeric(ctx Context) (*ZVal, error) {
	switch n := z.v.(type) {
	case ZInt:
		return z, nil
	case ZFloat:
		return z, nil
	case ZString:
		if strings.IndexAny(string(n), ".eE") >= 0 {
			// this is likely a float
			return z.As(ctx, ZtFloat)
		} else {
			return z.As(ctx, ZtInt)
		}
	default:
		return z.As(ctx, ZtInt)
	}
}

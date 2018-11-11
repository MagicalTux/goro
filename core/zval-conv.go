package core

import (
	"fmt"
	"strings"
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
	if z.GetType() == t {
		// nothing to do
		return z, nil
	}
	if t == ZtNull {
		// cast to NULL can only result into null
		return &ZVal{ZNull{}}, nil
	}

	if z.v == nil {
		v, err := ZNull{}.As(ctx, t)
		if err != nil {
			return nil, err
		}
		return v.ZVal(), nil
	}

	v, err := z.v.As(ctx, t)
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

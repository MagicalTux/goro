package phpv

import (
	"fmt"
)

// Used to make struct Callables satisfy the Val interface
type CallableVal struct{}

func (c CallableVal) GetType() ZType { return ZtCallable }

func (c CallableVal) ZVal() *ZVal { return NewZVal(c) }

func (c CallableVal) Value() Val { return c }

func (c CallableVal) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtCallable:
		return c, nil
	case ZtString:
		return ZStr("Callable"), nil
	case ZtInt:
		return ZInt(1), nil
	case ZtFloat:
		return ZFloat(1), nil
	case ZtBool:
		return ZBool(true), nil
	}
	return nil, fmt.Errorf("Cannot cast Callable to type %s", t.String())
}

func (c CallableVal) String() string {
	return "Callable"
}

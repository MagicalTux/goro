package phpv

import (
	"fmt"
)

// Used to make struct Callables satisfy the Val interface
type CallableVal struct{}

func (c CallableVal) Name() string { return "" }

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

type BoundedCallable struct {
	Callable
	This ZObject
	Args []*ZVal
}

type MethodCallable struct {
	Callable
	Class  ZClass
	Static bool
}

func Bind(fn Callable, this ZObject, args ...*ZVal) *BoundedCallable {
	return &BoundedCallable{fn, this, args}
}

func (b *BoundedCallable) GetArgs() []*FuncArg {
	if fga, ok := b.Callable.(FuncGetArgs); ok {
		return fga.GetArgs()
	}
	return nil
}

func (m *MethodCallable) Loc() *Loc {
	if loc, ok := m.Callable.(interface{ Loc() *Loc }); ok {
		return loc.Loc()
	}
	return nil
}

func BindClass(fn Callable, class ZClass, static bool) *MethodCallable {
	return &MethodCallable{fn, class, static}
}

func (m *MethodCallable) GetArgs() []*FuncArg {
	if fga, ok := m.Callable.(FuncGetArgs); ok {
		return fga.GetArgs()
	}
	return nil
}

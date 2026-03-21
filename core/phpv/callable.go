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

// Override Val methods so that BoundedCallable wraps itself in ZVal
// instead of delegating to the embedded Callable (which would lose the wrapper).
func (b *BoundedCallable) GetType() ZType                          { return ZtCallable }
func (b *BoundedCallable) ZVal() *ZVal                             { return NewZVal(b) }
func (b *BoundedCallable) Value() Val                              { return b }
func (b *BoundedCallable) AsVal(ctx Context, t ZType) (Val, error) { return CallableVal{}.AsVal(ctx, t) }
func (b *BoundedCallable) String() string                          { return "Callable" }

type MethodCallable struct {
	Callable
	Class       ZClass
	CalledClass ZClass // for late static binding; nil means same as Class
	Static      bool
	AliasName   string // non-empty when the method was called via a trait alias
}

// Override Val methods so that MethodCallable wraps itself in ZVal properly.
func (m *MethodCallable) GetType() ZType                          { return ZtCallable }
func (m *MethodCallable) ZVal() *ZVal                             { return NewZVal(m) }
func (m *MethodCallable) Value() Val                              { return m }
func (m *MethodCallable) AsVal(ctx Context, t ZType) (Val, error) { return CallableVal{}.AsVal(ctx, t) }
func (m *MethodCallable) String() string                          { return "Callable" }

func (m *MethodCallable) Name() string {
	if m.AliasName != "" {
		return m.AliasName
	}
	return m.Callable.Name()
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
	return &MethodCallable{fn, class, nil, static, ""}
}

// BindClassLSB creates a method callable with separate defining and called classes
// for late static binding support.
func BindClassLSB(fn Callable, definingClass ZClass, calledClass ZClass, static bool) *MethodCallable {
	return &MethodCallable{fn, definingClass, calledClass, static, ""}
}

func (m *MethodCallable) GetArgs() []*FuncArg {
	if fga, ok := m.Callable.(FuncGetArgs); ok {
		return fga.GetArgs()
	}
	return nil
}

func (m *MethodCallable) ReturnsByRef() bool {
	if rr, ok := m.Callable.(interface{ ReturnsByRef() bool }); ok {
		return rr.ReturnsByRef()
	}
	return false
}

func (b *BoundedCallable) ReturnsByRef() bool {
	if rr, ok := b.Callable.(interface{ ReturnsByRef() bool }); ok {
		return rr.ReturnsByRef()
	}
	return false
}

// DisplayName returns the fully-qualified callable name for display purposes
// (e.g. ob_get_status, ob_list_handlers). Unlike Name(), this includes
// the class prefix for methods.
func (m *MethodCallable) DisplayName() string {
	return string(m.Class.GetName()) + "::" + m.Callable.Name()
}

func (b *BoundedCallable) DisplayName() string {
	if b.This == nil {
		return b.Callable.Name()
	}
	return string(b.This.GetClass().GetName()) + "::" + b.Callable.Name()
}

// CallableDisplayName returns the display name for a Callable.
// For MethodCallable/BoundedCallable it includes the class prefix.
// For other callables it returns Name().
func CallableDisplayName(c Callable) string {
	if m, ok := c.(interface{ DisplayName() string }); ok {
		return m.DisplayName()
	}
	return c.Name()
}

// HookCallable wraps a Runnable (property hook body) as a Callable so it can be
// executed via CallZVal with a proper FuncContext (which sets $this, etc).
// The hook body uses "return" statements to produce a value; the FuncContext's
// CatchReturn mechanism captures that.
type HookCallable struct {
	CallableVal
	Hook     Runnable
	HookName string // e.g. "MyClass::$prop::get"
	Params   []*FuncArg
}

func (h *HookCallable) Name() string { return h.HookName }

func (h *HookCallable) Call(ctx Context, args []*ZVal) (*ZVal, error) {
	// Set parameter variables in the local scope (like ZClosure.callBody does)
	for i, p := range h.Params {
		if i < len(args) && args[i] != nil {
			ctx.OffsetSet(ctx, p.VarName.ZVal(), args[i])
		}
	}
	return h.Hook.Run(ctx)
}

func (h *HookCallable) GetArgs() []*FuncArg {
	return h.Params
}

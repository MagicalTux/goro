package phpobj

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

type NativeMethod func(ctx phpv.Context, this *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error)

func (m NativeMethod) Name() string { return "" }

func (m NativeMethod) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	this := ctx.This()
	if this == nil {
		// Allow static calls on NativeMethod - pass nil object.
		// The engine already checks IsStatic() before allowing :: calls,
		// so if we reach here it's a legitimate static method call.
		return m(ctx, nil, args)
	}

	return m(ctx, this.(*ZObject), args)
}

func (m NativeMethod) GetType() phpv.ZType { return phpv.ZtCallable }
func (m NativeMethod) ZVal() *phpv.ZVal    { return phpv.NewZVal(m) }
func (m NativeMethod) Value() phpv.Val     { return m }
func (m NativeMethod) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	if t == phpv.ZtCallable {
		return m, nil
	}
	return nil, errors.New("Cannot cast callables to other type")
}
func (m NativeMethod) String() string {
	return "Callable"
}

// NativeMethodNamed wraps a NativeMethod with parameter name metadata,
// enabling PHP 8.0 named argument support for native methods.
type NativeMethodNamed struct {
	Fn   NativeMethod
	Args []*phpv.FuncArg
}

func (m *NativeMethodNamed) Name() string { return "" }

func (m *NativeMethodNamed) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return m.Fn.Call(ctx, args)
}

func (m *NativeMethodNamed) GetArgs() []*phpv.FuncArg {
	return m.Args
}

func (m *NativeMethodNamed) GetType() phpv.ZType { return phpv.ZtCallable }
func (m *NativeMethodNamed) ZVal() *phpv.ZVal    { return phpv.NewZVal(m) }
func (m *NativeMethodNamed) Value() phpv.Val     { return m }
func (m *NativeMethodNamed) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	if t == phpv.ZtCallable {
		return m, nil
	}
	return nil, errors.New("Cannot cast callables to other type")
}
func (m *NativeMethodNamed) String() string {
	return "Callable"
}

// namedCallable wraps any Callable with an explicit name for stack trace display.
// Used by GetMethod to preserve the declared method name for native methods that
// return "" from Name() (e.g. NativeMethod, NativeStaticMethod, NativeMethodNamed).
type namedCallable struct {
	phpv.Callable
	name string
}

func (n *namedCallable) Name() string { return n.name }
func (n *namedCallable) GetType() phpv.ZType { return phpv.ZtCallable }
func (n *namedCallable) ZVal() *phpv.ZVal    { return phpv.NewZVal(n) }
func (n *namedCallable) Value() phpv.Val     { return n }
func (n *namedCallable) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	return n.Callable.AsVal(ctx, t)
}

// NativeStaticMethod is like NativeMethod but for static methods.
// It receives the class from context instead of requiring $this.
type NativeStaticMethod func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)

func (m NativeStaticMethod) Name() string { return "" }

func (m NativeStaticMethod) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return m(ctx, args)
}

func (m NativeStaticMethod) GetType() phpv.ZType { return phpv.ZtCallable }
func (m NativeStaticMethod) ZVal() *phpv.ZVal    { return phpv.NewZVal(m) }
func (m NativeStaticMethod) Value() phpv.Val     { return m }
func (m NativeStaticMethod) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	if t == phpv.ZtCallable {
		return m, nil
	}
	return nil, errors.New("Cannot cast callables to other type")
}
func (m NativeStaticMethod) String() string {
	return "Callable"
}

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
		return nil, ctx.Errorf("Non-static method cannot be called statically")
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

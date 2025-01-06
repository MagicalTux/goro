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

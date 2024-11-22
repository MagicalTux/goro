package phpobj

import (
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

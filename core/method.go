package core

import "errors"

type NativeMethod func(ctx Context, this *ZObject, args []*ZVal) (*ZVal, error)

func (m NativeMethod) Call(ctx Context, args []*ZVal) (*ZVal, error) {
	this := ctx.This()
	if this == nil {
		return nil, errors.New("Non-static method cannot be called statically")
	}

	return m(ctx, this, args)
}

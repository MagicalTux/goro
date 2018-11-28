package core

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

func SpawnCallable(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error) {
	switch v.GetType() {
	case phpv.ZtString:
		// name of a method
		s := v.Value().(phpv.ZString)
		return ctx.Global().(*Global).GetFunction(ctx, s)
		// TODO handle ZtObject (call __invoke, handle closures too)
		// TODO handle ZtArray (object, method, or class_name, method)
	default:
		// TODO error
		return nil, errors.New("Argument %d passed to %s() must be callable, integer given")
	}
}

type callCatcher struct {
	name   phpv.ZString
	target phpv.Callable
}

func (c *callCatcher) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()
	for _, sub := range args {
		a.OffsetSet(ctx, nil, sub)
	}
	rArgs := []*phpv.ZVal{c.name.ZVal(), a.ZVal()}

	return c.target.Call(ctx, rArgs)
}

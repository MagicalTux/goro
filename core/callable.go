package core

import "errors"

func SpawnCallable(ctx Context, v *ZVal) (Callable, error) {
	switch v.GetType() {
	case ZtString:
		// name of a method
		s := v.Value().(ZString)
		return ctx.Global().GetFunction(s)
		// TODO handle ZtObject (call __invoke, handle closures too)
		// TODO handle ZtArray (object, method, or class_name, method)
	default:
		// TODO error
		return nil, errors.New("Argument %d passed to %s() must be callable, integer given")
	}
}

type callCatcher struct {
	name   ZString
	target Callable
}

func (c *callCatcher) Call(ctx Context, args []*ZVal) (*ZVal, error) {
	a := NewZArray()
	for _, sub := range args {
		a.OffsetSet(ctx, nil, sub)
	}
	rArgs := []*ZVal{c.name.ZVal(), a.ZVal()}

	return c.target.Call(ctx, rArgs)
}

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
		return ctx.Global().GetFunction(ctx, s)
		// TODO handle ZtObject (call __invoke, handle closures too)
		// TODO handle ZtArray (object, method, or class_name, method)
	default:
		// TODO error
		return nil, errors.New("Argument %d passed to %s() must be callable, integer given")
	}
}

package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func void spl_autoload_register ([ callable $autoload_function [, bool $throw = true [, bool $prepend = false ]]] )
func fncSplAutoloadRegister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	ctx.Global().RegisterAutoload(handler)
	return nil, nil
}

// > func bool spl_autoload_unregister ( mixed $autoload_function )
func fncSplAutoloadUnregister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	result := ctx.Global().UnregisterAutoload(handler)
	return phpv.ZBool(result).ZVal(), nil
}

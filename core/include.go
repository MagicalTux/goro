package core

import "github.com/MagicalTux/goro/core/phpv"

// > func mixed include (string filename)
func fncInclude(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1)
	return ctx.Global().Include(ctx, fn)
}

// > func mixed require (string filename)
func fncRequire(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1)
	return ctx.Global().Require(ctx, fn)
}

// > func mixed include_once (string filename)
func fncIncludeOnce(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1)
	return ctx.Global().IncludeOnce(ctx, fn)
}

// > func mixed require_once (string filename)
func fncRequireOnce(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1)
	return ctx.Global().RequireOnce(ctx, fn)
}

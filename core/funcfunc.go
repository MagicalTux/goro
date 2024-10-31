package core

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// Function handling Functions

// > func array func_get_args ( void )
func fncFuncGetArgs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// no params

	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, errors.New("func_get_args():  Called from the global scope - no function context")
	}

	r := phpv.NewZArray()

	for _, v := range c.Args {
		r.OffsetSet(ctx, nil, v)
	}

	return r.ZVal(), nil
}

// > func int func_num_args ( void )
func fncFuncNumArgs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, errors.New("func_num_args():  Called from the global scope - no function context")
	}

	return phpv.ZInt(len(c.Args)).ZVal(), nil
}

// > func mixed func_get_arg ( int $arg_num )
func fncFuncGetArg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var argNum phpv.ZInt
	_, err := Expand(ctx, args, &argNum)
	if err != nil {
		return nil, err
	}

	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, errors.New("func_get_arg():  Called from the global scope - no function context")
	}

	if argNum < 0 || argNum >= phpv.ZInt(len(c.Args)) {
		return phpv.ZNull{}.ZVal(), nil
	}

	return c.Args[argNum], nil
}

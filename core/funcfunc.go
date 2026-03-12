package core

import (
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Function handling Functions

// > func array func_get_args ( void )
func fncFuncGetArgs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// no params

	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_get_args() cannot be called from the global scope")
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_num_args() must be called from a function context")
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_get_arg() cannot be called from the global scope")
	}

	if argNum < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "func_get_arg(): Argument #1 ($position) must be greater than or equal to 0")
	}
	if argNum >= phpv.ZInt(len(c.Args)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "func_get_arg(): Argument #1 ($position) must be less than the number of the arguments passed to the currently executed function")
	}

	// PHP 7+: func_get_arg returns the CURRENT value of the parameter
	// (after any modifications in the function body), not the original
	type argsGetter interface {
		GetArgs() []*phpv.FuncArg
	}
	callable := c.Callable()
	if ag, ok := callable.(argsGetter); ok {
		funcArgs := ag.GetArgs()
		if int(argNum) < len(funcArgs) {
			// Get current value from the function context's variables
			v, err := c.OffsetGet(ctx, funcArgs[argNum].VarName)
			if err == nil && v != nil {
				return v, nil
			}
		}
	}

	return c.Args[argNum], nil
}

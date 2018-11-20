package core

import "errors"

// Function handling Functions

//> func array func_get_args ( void )
func fncFuncGetArgs(ctx Context, args []*ZVal) (*ZVal, error) {
	// no params

	// go back one context
	c, ok := ctx.Parent(1).(*FuncContext)
	if !ok {
		return nil, errors.New("func_get_args():  Called from the global scope - no function context")
	}

	r := NewZArray()

	for _, v := range c.args {
		r.OffsetSet(ctx, nil, v)
	}

	return r.ZVal(), nil
}

//> func int func_num_args ( void )
func fncFuncNumArgs(ctx Context, args []*ZVal) (*ZVal, error) {
	// go back one context
	c, ok := ctx.Parent(1).(*FuncContext)
	if !ok {
		return nil, errors.New("func_num_args():  Called from the global scope - no function context")
	}

	return ZInt(len(c.args)).ZVal(), nil
}

//> func mixed func_get_arg ( int $arg_num )
func fncFuncGetArg(ctx Context, args []*ZVal) (*ZVal, error) {
	var argNum ZInt
	_, err := Expand(ctx, args, &argNum)
	if err != nil {
		return nil, err
	}

	// go back one context
	c, ok := ctx.Parent(1).(*FuncContext)
	if !ok {
		return nil, errors.New("func_get_arg():  Called from the global scope - no function context")
	}

	if argNum < 0 || argNum >= ZInt(len(c.args)) {
		return ZNull{}.ZVal(), nil
	}

	return c.args[argNum], nil
}

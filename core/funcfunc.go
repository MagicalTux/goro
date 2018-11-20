package core

import "errors"

// Function handling Functions

//> func array func_get_args ( void )
func fncFuncGetArgs(ctx Context, args []*ZVal) (*ZVal, error) {
	// no params

	c, ok := ctx.(*FuncContext)
	if !ok {
		return nil, errors.New("func_get_args():  Called from the global scope - no function context")
	}

	// go back one context
	c, ok = c.Context.(*FuncContext)
	if !ok {
		return nil, errors.New("func_get_args():  Called from the global scope - no function context")
	}

	r := NewZArray()

	for _, v := range c.args {
		r.OffsetSet(ctx, nil, v)
	}

	return r.ZVal(), nil
}

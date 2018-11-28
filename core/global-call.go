package core

import "github.com/MagicalTux/goro/core/phpv"

// perform call in new context
func (c *Global) Call(ctx phpv.Context, f phpv.Callable, args []phpv.Runnable, this phpv.Val) (*phpv.ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       phpv.NewHashTable(),
		c:       f,
	}
	if this != nil {
		callCtx.this = this.(*ZObject)
	}

	var func_args []*funcArg
	if c, ok := f.(funcGetArgs); ok {
		func_args = c.getArgs()
	}

	// collect args
	// use func_args to check if any arg is a ref and needs to be passed as such
	var err error
	callCtx.args = make([]*phpv.ZVal, len(args))
	for i, a := range args {
		callCtx.args[i], err = a.Run(ctx)
		if err != nil {
			return nil, err
		}
		if i < len(func_args) && func_args[i].ref {
			callCtx.args[i] = callCtx.args[i].Ref()
		} else {
			callCtx.args[i] = callCtx.args[i].Dup()
		}
	}

	return CatchReturn(f.Call(callCtx, callCtx.args))
}

func (c *Global) CallZVal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, this phpv.Val) (*phpv.ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       phpv.NewHashTable(),
		args:    args,
		c:       f,
	}
	if this != nil {
		callCtx.this = this.(*ZObject)
	}

	return CatchReturn(f.Call(callCtx, args))
}

func (c *Global) Parent(n int) phpv.Context {
	return nil
}

package core

// perform call in new context
func (c *Global) Call(ctx Context, f Callable, args []Runnable, this *ZObject) (*ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       NewHashTable(),
		this:    this,
		c:       f,
	}

	var func_args []*funcArg
	if c, ok := f.(funcGetArgs); ok {
		func_args = c.getArgs()
	}

	// collect args
	// use func_args to check if any arg is a ref and needs to be passed as such
	var err error
	callCtx.args = make([]*ZVal, len(args))
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

func (c *Global) CallZVal(ctx Context, f Callable, args []*ZVal, this *ZObject) (*ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       NewHashTable(),
		this:    this,
		args:    args,
		c:       f,
	}

	return CatchReturn(f.Call(callCtx, args))
}

func (c *Global) Parent(n int) Context {
	return nil
}

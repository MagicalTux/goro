package phpctx

import (
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// perform call in new context
func (c *Global) Call(ctx phpv.Context, f phpv.Callable, args []phpv.Runnable, this phpv.ZObject) (*phpv.ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       phpv.NewHashTable(),
		c:       f,
	}
	if this != nil {
		callCtx.this = this
	}

	var func_args []*phpv.FuncArg
	if c, ok := f.(phpv.FuncGetArgs); ok {
		func_args = c.GetArgs()
	}

	// collect args
	// use func_args to check if any arg is a ref and needs to be passed as such
	var err error
	callCtx.Args = make([]*phpv.ZVal, len(args))
	for i, a := range args {
		callCtx.Args[i], err = a.Run(ctx)
		if err != nil {
			return nil, err
		}
		if i < len(func_args) && func_args[i].Ref {
			callCtx.Args[i] = callCtx.Args[i].Ref()
		} else {
			callCtx.Args[i] = callCtx.Args[i].Dup()
		}
	}

	c.callStack = append(c.callStack, f)
	defer func() {
		c.callStack = c.callStack[0 : len(c.callStack)-1]
	}()

	return phperr.CatchReturn(f.Call(callCtx, callCtx.Args))
}

func (c *Global) CallZVal(ctx phpv.Context, f phpv.Callable, args []*phpv.ZVal, this phpv.ZObject) (*phpv.ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       phpv.NewHashTable(),
		Args:    args,
		c:       f,
	}
	if this != nil {
		callCtx.this = this
	}

	return phperr.CatchReturn(f.Call(callCtx, args))
}

func (c *Global) Parent(n int) phpv.Context {
	return nil
}

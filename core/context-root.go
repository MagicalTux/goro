package core

import (
	"context"
	"errors"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type RootContext struct {
	context.Context

	h *ZHashTable
	g *Global
}

func (c *RootContext) AsVal(ctx Context, t ZType) (Val, error) {
	a := &ZArray{c.h, false}
	return a.AsVal(ctx, t)
}

func (c *RootContext) GetType() ZType {
	return ZtArray
}

func (c *RootContext) ZVal() *ZVal {
	return (&ZVal{c}).Ref()
}

func (c *RootContext) Global() *Global {
	return c.g
}

func (c *RootContext) Root() *RootContext {
	return c
}

func (c *RootContext) OffsetExists(ctx Context, name *ZVal) (bool, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return false, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		return true, nil
	}

	return c.h.HasString(name.AsString(ctx)), nil
}

func (c *RootContext) OffsetGet(ctx Context, name *ZVal) (*ZVal, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return nil, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		v, err := c.AsVal(ctx, ZtArray)
		return v.ZVal(), err
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *RootContext) OffsetSet(ctx Context, name, v *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name.AsString(ctx), v)
}

func (c *RootContext) OffsetUnset(ctx Context, name *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(name.AsString(ctx))
}

func (c *RootContext) Count(ctx Context) ZInt {
	return c.h.count
}

func (c *RootContext) NewIterator() ZIterator {
	return c.h.NewIterator()
}

func (c *RootContext) Include(ctx Context, fn ZString) (*ZVal, error) {
	f, err := ctx.Global().Open(fn)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = ZString(fn2)
	}

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return code.Run(ctx)
}

func (c *RootContext) GetConfig(name ZString, def *ZVal) *ZVal {
	return c.g.GetConfig(name, def)
}

func (c *RootContext) Write(v []byte) (int, error) {
	return c.g.Write(v)
}

// perform call in new context
func (c *RootContext) Call(ctx Context, f Callable, args []Runnable, this *ZObject) (*ZVal, error) {
	callCtx := &FuncContext{
		Context: ctx,
		h:       NewHashTable(),
		this:    this,
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

	return f.Call(callCtx, callCtx.args)
}

func (c *RootContext) Parent(n int) Context {
	return nil
}

package core

import (
	"errors"
)

type FuncContext struct {
	Context

	h    *ZHashTable
	this *ZObject
	args []*ZVal
}

func (c *FuncContext) AsVal(ctx Context, t ZType) (Val, error) {
	a := &ZArray{c.h, false}
	return a.AsVal(ctx, t)
}

func (c *FuncContext) GetType() ZType {
	return ZtArray
}

func (c *FuncContext) ZVal() *ZVal {
	return (&ZVal{c}).Ref()
}

func (c *FuncContext) OffsetExists(ctx Context, name *ZVal) (bool, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return false, err
	}

	switch name.AsString(ctx) {
	case "this":
		if c.this == nil {
			return false, nil
		}
		return true, nil
	case "GLOBALS":
		return true, nil
	case "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Root().OffsetExists(ctx, name)
	}
	return c.h.HasString(name.AsString(ctx)), nil
}

func (c *FuncContext) OffsetGet(ctx Context, name *ZVal) (*ZVal, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return nil, err
	}

	switch name.AsString(ctx) {
	case "this":
		if c.this == nil {
			return nil, nil
		}
		return c.this.ZVal(), nil
	case "GLOBALS", "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Root().OffsetGet(ctx, name)
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *FuncContext) OffsetSet(ctx Context, name, v *ZVal) error {
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

func (c *FuncContext) OffsetUnset(ctx Context, name *ZVal) error {
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

func (c *FuncContext) Count(ctx Context) ZInt {
	return c.h.count
}

func (c *FuncContext) NewIterator() ZIterator {
	return c.h.NewIterator()
}

func (ctx *FuncContext) Parent(n int) Context {
	if n <= 1 {
		return ctx.Context
	} else {
		return ctx.Context.Parent(n - 1)
	}
}

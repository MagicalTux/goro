package core

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

type FuncContext struct {
	phpv.Context

	h    *phpv.ZHashTable
	this *ZObject
	args []*phpv.ZVal
	c    phpv.Callable // called object (this function itself)
}

func (c *FuncContext) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	a := c.h.Array()
	return a.AsVal(ctx, t)
}

func (c *FuncContext) GetType() phpv.ZType {
	return phpv.ZtArray
}

func (c *FuncContext) ZVal() *phpv.ZVal {
	return c.ZVal().Ref()
}

func (c *FuncContext) Func() phpv.Context {
	return c
}

func (c *FuncContext) This() phpv.Val {
	if c.this != nil {
		return c.this
	}
	return c.Context.This()
}

func (c *FuncContext) OffsetExists(ctx phpv.Context, name *phpv.ZVal) (bool, error) {
	name, err := name.As(ctx, phpv.ZtString)
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
		return c.Global().OffsetExists(ctx, name)
	}
	return c.h.HasString(name.AsString(ctx)), nil
}

func (c *FuncContext) OffsetGet(ctx phpv.Context, name *phpv.ZVal) (*phpv.ZVal, error) {
	name, err := name.As(ctx, phpv.ZtString)
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
		return c.Global().OffsetGet(ctx, name)
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *FuncContext) OffsetSet(ctx phpv.Context, name, v *phpv.ZVal) error {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name.AsString(ctx), v)
}

func (c *FuncContext) OffsetUnset(ctx phpv.Context, name *phpv.ZVal) error {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(name.AsString(ctx))
}

func (c *FuncContext) Count(ctx phpv.Context) phpv.ZInt {
	return c.h.Count()
}

func (c *FuncContext) NewIterator() phpv.ZIterator {
	return c.h.NewIterator()
}

func (ctx *FuncContext) Parent(n int) phpv.Context {
	if n <= 1 {
		return ctx.Context
	} else {
		return ctx.Context.Parent(n - 1)
	}
}

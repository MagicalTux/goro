package core

import (
	"context"
	"errors"
	"net/url"

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

func (ctx *RootContext) Include(_fn ZString) (*ZVal, error) {
	fn := string(_fn)
	u, err := url.Parse(fn)
	if err != nil {
		return nil, err
	}

	f, err := ctx.Global().p.Open(u)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = fn2
	}

	// tokenize
	t := tokenizer.NewLexer(f, fn)

	// compile
	c, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return c.Run(ctx)
}

func (c *RootContext) GetConfig(name ZString, def *ZVal) *ZVal {
	return c.g.GetConfig(name, def)
}

func (c *RootContext) Write(v []byte) (int, error) {
	return c.g.Write(v)
}

package core

import (
	"errors"
	"net/url"

	"github.com/MagicalTux/gophp/core/tokenizer"
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

func (ctx *FuncContext) Include(_fn ZString) (*ZVal, error) {
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

func (ctx *FuncContext) Parent(n int) Context {
	if n <= 1 {
		return ctx.Context
	} else {
		return ctx.Context.Parent(n - 1)
	}
}

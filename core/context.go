package core

import (
	"context"
	"errors"
	"io"
	"net/url"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type Context interface {
	context.Context
	ZArrayAccess
	io.Writer

	GetGlobal() *Global

	GetFunction(name ZString) (Callable, error)
	RegisterFunction(name ZString, f Callable) error

	GetConfig(name ZString, def *ZVal) *ZVal

	Include(fn ZString) (*ZVal, error)
}

type phpContext struct {
	Context

	h    *ZHashTable
	this *ZObject
}

func NewContext(parent Context) Context {
	return &phpContext{
		Context: parent,
		h:       NewHashTable(),
	}
}

func NewContextWithObject(parent Context, this *ZObject) Context {
	return &phpContext{
		Context: parent,
		h:       NewHashTable(),
		this:    this,
	}
	//ctx.SetVariable("this", o.ZVal())
}

func (c *phpContext) AsVal(ctx Context, t ZType) (Val, error) {
	a := &ZArray{c.h, false}
	return a.AsVal(ctx, t)
}

func (c *phpContext) GetType() ZType {
	return ZtArray
}

func (c *phpContext) ZVal() *ZVal {
	return (&ZVal{c}).Ref()
}

func (c *phpContext) OffsetGet(ctx Context, name *ZVal) (*ZVal, error) {
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
		return c.GetGlobal().OffsetGet(ctx, name)
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *phpContext) OffsetSet(ctx Context, name, v *ZVal) error {
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

func (ctx *phpContext) Include(_fn ZString) (*ZVal, error) {
	fn := string(_fn)
	u, err := url.Parse(fn)
	if err != nil {
		return nil, err
	}

	f, err := ctx.GetGlobal().p.Open(u)
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
	c := Compile(ctx, t)

	var z *ZVal
	z, err = c.Run(ctx)
	if e, ok := err.(*PhpError); ok {
		switch e.t {
		case PhpExit:
			return z, nil
		}
	}
	return z, err
}

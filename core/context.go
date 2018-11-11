package core

import (
	"context"
	"errors"
	"io"
	"net/url"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Context interface {
	context.Context
	io.Writer

	GetGlobal() *Global

	GetFunction(name ZString) (Callable, error)
	RegisterFunction(name ZString, f Callable) error

	GetVariable(name ZString) (*ZVal, error)
	SetVariable(name ZString, v *ZVal) error

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

func (c *phpContext) GetVariable(name ZString) (*ZVal, error) {
	switch name {
	case "this":
		return c.this.ZVal(), nil
	}
	return c.h.GetString(name), nil
}

func (c *phpContext) SetVariable(name ZString, v *ZVal) error {
	switch name {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name, v)
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

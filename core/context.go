package core

import (
	"context"
	"errors"
	"io"
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

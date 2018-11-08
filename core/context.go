package core

import (
	"context"
	"io"
)

type Context interface {
	context.Context
	io.Writer

	GetFunction(name ZString) (Callable, error)
	RegisterFunction(name ZString, f Callable) error

	GetVariable(name ZString) (*ZVal, error)
	SetVariable(name ZString, v *ZVal) error

	GetConstant(name ZString) (*ZVal, error)
}

type phpContext struct {
	Context

	h *ZHashTable
}

func NewContext(parent Context) Context {
	return &phpContext{
		Context: parent,
		h:       NewHashTable(),
	}
}

func (c *phpContext) GetVariable(name ZString) (*ZVal, error) {
	return c.h.GetString(name), nil
}

func (c *phpContext) SetVariable(name ZString, v *ZVal) error {
	return c.h.SetString(name, v)
}

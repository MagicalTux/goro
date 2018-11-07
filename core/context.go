package core

import (
	"context"
	"io"
)

type Context interface {
	context.Context
	io.Writer

	GetFunction(name string) (Callable, error)
	RegisterFunction(name string, f Callable) error

	GetVariable(name string) (*ZVal, error)
	SetVariable(name string, v *ZVal) error
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

func (c *phpContext) GetVariable(name string) (*ZVal, error) {
	return c.h.GetString(name), nil
}

func (c *phpContext) SetVariable(name string, v *ZVal) error {
	return c.h.SetString(name, v)
}

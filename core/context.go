package core

import (
	"context"
	"io"
)

type Context interface {
	context.Context
	io.Writer

	GetVariable(name string) (*ZVal, error)
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

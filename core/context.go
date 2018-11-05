package core

import (
	"context"
	"io"
)

type Context interface {
	context.Context
	io.Writer
}

type phpContext struct {
	Context

	h *ZHashTable
}

func NewContext(parent Context) Context {
	return &phpContext{
		Context: parent,
	}
}

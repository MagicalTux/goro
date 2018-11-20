package core

import (
	"context"
	"io"
)

type Context interface {
	context.Context
	ZArrayAccess
	ZCountable
	ZIterable
	io.Writer

	Global() *Global
	Root() *RootContext

	GetConfig(name ZString, def *ZVal) *ZVal

	Include(fn ZString) (*ZVal, error)

	Call(ctx Context, f Callable, args []Runnable, this *ZObject) (*ZVal, error)
}

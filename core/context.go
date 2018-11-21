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
	Parent(n int) Context

	GetConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this *ZObject) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this *ZObject) (*ZVal, error)
}

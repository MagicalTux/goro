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
	Func() *FuncContext
	Parent(n int) Context
	This() *ZObject
	Loc() *Loc
	Tick(ctx Context, l *Loc) error

	GetConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this *ZObject) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this *ZObject) (*ZVal, error)
}

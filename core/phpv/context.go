package phpv

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

	Global() Context
	Func() Context
	Parent(n int) Context
	This() Val
	Loc() *Loc
	Tick(ctx Context, l *Loc) error
	MemAlloc(ctx Context, s uint64) error

	GetConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this Val) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this Val) (*ZVal, error)
}

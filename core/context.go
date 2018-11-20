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
}

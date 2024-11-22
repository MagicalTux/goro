package phpv

import (
	"context"
	"io"

	"github.com/MagicalTux/goro/core/stream"
)

type Context interface {
	context.Context
	ZArrayAccess
	ZCountable
	ZIterable
	io.Writer

	Global() GlobalContext
	Func() FuncContext
	Parent(n int) Context
	This() ZObject
	Loc() *Loc
	Tick(ctx Context, l *Loc) error
	MemAlloc(ctx Context, s uint64) error

	Errorf(format string, a ...any) error
	Error(err error, t ...PhpErrorType) error
	FuncErrorf(format string, a ...any) error
	FuncError(err error, t ...PhpErrorType) error

	GetFuncName() string

	GetConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this ZObject) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this ZObject) (*ZVal, error)
}

type GlobalContext interface {
	Context

	Flush()

	RegisterFunction(name ZString, f Callable) error
	GetFunction(ctx Context, name ZString) (Callable, error)

	RegisterClass(name ZString, c ZClass) error
	GetClass(ctx Context, name ZString, autoload bool) (ZClass, error)
	SetLocalConfig(name ZString, val *ZVal) error

	ConstantSet(k ZString, v Val) bool
	ConstantGet(k ZString) (Val, bool)

	RegisterLazyFunc(name ZString, r Runnables, p int)
	RegisterLazyClass(name ZString, r Runnables, p int)

	Open(fn ZString, isInclude bool) (*stream.Stream, error)
	Exists(fn ZString) (bool, error)
	Chdir(d ZString) error
	Getwd() ZString

	Getenv(key string) (string, bool)
	Setenv(key, value string) error
	Unsetenv(key string) error

	Include(ctx Context, fn ZString) (*ZVal, error)
	Require(ctx Context, fn ZString) (*ZVal, error)
	IncludeOnce(ctx Context, fn ZString) (*ZVal, error)
	RequireOnce(ctx Context, fn ZString) (*ZVal, error)
}

type FuncContext interface {
	Context
}

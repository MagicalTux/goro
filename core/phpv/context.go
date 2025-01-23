package phpv

import (
	"context"
	"io"
	"iter"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/random"
	"github.com/MagicalTux/goro/core/stream"
)

type Context interface {
	context.Context
	ZArrayAccess
	ZCountable
	ZIterable
	io.Writer

	// return value of GetScriptFile will change depending on which
	// currently include()'d or require()'d file is running
	GetScriptFile() ZString

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

	// In the following functions, args can also take logopt types:
	// examples:
	//   Warn("testing %d", 123, logopt.NoFuncName(true))
	//   Notice("note %s", "asdf", logopt.NoLoc(true))
	//   Notice("nope", logopt.Data{NoLoc: false})
	Warn(format string, args ...any) error
	Notice(format string, args ...any) error
	Deprecated(format string, args ...any) error

	LogError(err *PhpError, optionArg ...logopt.Data)

	WarnDeprecated() error

	GetFuncName() string

	GetConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this ...ZObject) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this ...ZObject) (*ZVal, error)
}

type GlobalContext interface {
	Context

	Flush()

	Argv() []string

	RegisterFunction(name ZString, f Callable) error
	GetFunction(ctx Context, name ZString) (Callable, error)

	RegisterClass(name ZString, c ZClass) error
	GetClass(ctx Context, name ZString, autoload bool) (ZClass, error)

	SetLocalConfig(name ZString, value *ZVal) error
	IterateConfig() iter.Seq2[string, IniValue]

	ConstantSet(k ZString, v Val) bool
	ConstantGet(k ZString) (Val, bool)

	RegisterLazyFunc(name ZString, r Runnables, p int)
	RegisterLazyClass(name ZString, r Runnables, p int)

	Open(fn ZString, useIncludePath bool) (*stream.Stream, error)
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

	GetLoadedExtensions() []string

	Random() *random.State

	GetUserErrorHandler() (Callable, PhpErrorType)
	SetUserErrorHandler(Callable, PhpErrorType)
}

type IniValue struct {
	Global *ZVal
	Local  *ZVal
}

type IniConfig interface {
	Get(name ZString) *IniValue
	SetLocal(name ZString, value *ZVal)
	IterateConfig() iter.Seq2[string, IniValue]
}

type FuncContext interface {
	Context
}

package phpv

import (
	"context"
	"io"
	"iter"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/random"
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
	Class() ZClass
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
	UserDeprecated(format string, args ...any) error

	LogError(err *PhpError, optionArg ...logopt.Data)

	WarnDeprecated() error

	GetFuncName() string

	GetConfig(name ZString, def *ZVal) *ZVal
	GetGlobalConfig(name ZString, def *ZVal) *ZVal

	Call(ctx Context, f Callable, args []Runnable, this ...ZObject) (*ZVal, error)
	CallZVal(ctx Context, f Callable, args []*ZVal, this ...ZObject) (*ZVal, error)
	CallZValInternal(ctx Context, f Callable, args []*ZVal, this ...ZObject) (*ZVal, error)

	GetStackTrace(ctx Context) []*StackTraceEntry

	HeaderContext() *HeaderContext
}

type GlobalContext interface {
	Context

	Flush()

	Argv() []string

	RegisterFunction(name ZString, f Callable) error
	GetFunction(ctx Context, name ZString) (Callable, error)

	RegisterShutdownFunction(f Callable)

	RegisterClass(name ZString, c ZClass) error
	UnregisterClass(name ZString)
	GetClass(ctx Context, name ZString, autoload bool) (ZClass, error)
	SetCompilingClass(c ZClass)
	GetCompilingClass() ZClass

	RegisterAutoload(handler Callable)
	UnregisterAutoload(handler Callable) bool
	GetAutoloadFunctions() []Callable

	RestoreConfig(name ZString)
	SetLocalConfig(name ZString, value *ZVal) (*ZVal, bool)
	IterateConfig() iter.Seq2[string, IniValue]

	ConstantSet(k ZString, v Val) bool
	ConstantGet(k ZString) (Val, bool)
	ConstantForceSet(k ZString, v Val) // overwrite even if already set
	ConstantSetAttributes(k ZString, attrs []*ZAttribute)
	ConstantGetAttributes(k ZString) []*ZAttribute

	RegisterLazyFunc(name ZString, r Runnables, p int)
	RegisterLazyClass(name ZString, r Runnables, p int)

	Open(ctx Context, fn, mode ZString, useIncludePath bool, streamCtx ...Resource) (Stream, error)
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
	RestoreUserErrorHandler()
	SetUserExceptionHandler(handler Callable, originalVal *ZVal) *ZVal
	RestoreUserExceptionHandler()

	WriteErr(p []byte) (n int, err error)
	ShownDeprecated(key string) bool

	NextResourceID() int
	NextObjectID() int
	ReleaseObjectID(id int)

	GetDeclaredClasses() []ZString
	GetDefinedFunctions(ctx Context, excludeDisabled bool) (*ZArray, error)

	RegisterDestructor(obj ZObject)
	UnregisterDestructor(obj ZObject)

	CheckOpenBasedir(ctx Context, path string, funcName string) error
	IsWithinOpenBasedir(path string) bool

	// OpenFile opens a file for reading through the global file access layer.
	// This centralizes file access so it can be scoped to an fs.FS in the future.
	// The caller must close the returned ReadCloser.
	OpenFile(ctx Context, path string) (io.ReadCloser, error)

	IsUploadedFile(path string) bool
	UnregisterUploadedFile(path string)

	GetIncludedFiles() []string

	LastCallable() Callable
	ClearLastCallable()

	RegisterTickFunction(cb Callable, args []*ZVal)
	UnregisterTickFunction(cb Callable)
	CallTickFunctions(ctx Context) error
	HasTickFunctions() bool
}

type FuncContext interface {
	Context
}

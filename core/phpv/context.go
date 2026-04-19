package phpv

import (
	"context"
	"io"
	"iter"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/random"
)

// StateKey is an opaque handle for a per-request state slot on the Global
// context. Extensions allocate one key per state item they need (typically a
// package-level var set via NewStateKey) and access the stored value through
// GlobalContext.State / SetState. Keys are compared by pointer identity, so
// two NewStateKey calls with the same name still produce distinct keys.
//
// This lets core/phpv stay ignorant of extension-specific types: the state
// store holds any values, and the extension package owns the typed wrapper
// that casts the stored any back to its own struct.
type StateKey struct {
	name string // purely for debug / stringer output
}

// NewStateKey returns a fresh state key. name is not semantically significant;
// it shows up in String() to aid debugging.
func NewStateKey(name string) *StateKey {
	return &StateKey{name: name}
}

func (k *StateKey) String() string {
	if k == nil {
		return "<nil StateKey>"
	}
	return k.name
}

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
	CallZValNoCalledIn(ctx Context, f Callable, args []*ZVal, this ...ZObject) (*ZVal, error)

	GetStackTrace(ctx Context) []*StackTraceEntry

	HeaderContext() *HeaderContext
}

// GeneratorCallerContext is implemented by generator execution contexts to provide
// the calling trace (the external call stack that resumed this generator).
// This allows GetStackTrace to build complete backtraces that include both the
// generator's own frames and the external invocation chain.
type GeneratorCallerContext interface {
	// GetCallingTrace returns the stack trace entries from the context that
	// resumed this generator, plus a synthetic Generator->method() frame.
	// The entries should be appended after the generator's own frames.
	GetCallingTrace() []*StackTraceEntry
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

	RegisterAutoload(handler Callable, prepend bool)
	UnregisterAutoload(handler Callable) bool
	UnregisterAutoloadByName(name string) bool
	ClearAutoloadFunctions()
	GetAutoloadFunctions() []Callable
	GetAutoloadExtensions() string
	SetAutoloadExtensions(exts string)

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

	// State returns the value previously stored under key, or nil if none.
	// Extensions allocate the key once (NewStateKey) and use State / SetState
	// to stash per-request data without teaching core about their types.
	State(key *StateKey) any
	SetState(key *StateKey, value any)

	GetUserErrorHandler() (Callable, PhpErrorType, *ZVal)
	SetUserErrorHandler(handler Callable, filter PhpErrorType, originalVal *ZVal)
	RestoreUserErrorHandler()
	SetUserExceptionHandler(handler Callable, originalVal *ZVal) *ZVal
	RestoreUserExceptionHandler()

	WriteErr(p []byte) (n int, err error)
	ShownDeprecated(key string) bool

	NextResourceID() int
	NextObjectID() int
	ReleaseObjectID(id int)

	// RegisterTempObject registers an object ID as a "temporary" that should be
	// released if it has refcount 0 at the next statement boundary.
	// isFree should return true if the object has no PHP references (refcount == 0).
	RegisterTempObject(id int, isFree func() bool)
	// DrainTempObjects checks all registered temporary objects and releases any
	// that are still unreferenced (refcount == 0). Called at statement boundaries.
	DrainTempObjects()

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

	// MemMgrTracker returns the memory tracker for PHP-level allocation tracking.
	MemMgrTracker() MemTracker

	LastCallable() Callable
	ClearLastCallable()

	RegisterTickFunction(cb Callable, args []*ZVal)
	UnregisterTickFunction(cb Callable)
	CallTickFunctions(ctx Context) error
	HasTickFunctions() bool

	SetStrictTypes(v bool)
	GetStrictTypes() bool

	IsFunctionDisabled(name ZString) bool

	SetNextCallSuppressCalledIn(v bool)

	SetSkipNextDynPropDeprecation(v bool)
	TakeSkipNextDynPropDeprecation() bool

	// WriteStartupWarning buffers a warning message emitted during request startup
	WriteStartupWarning(msg string)
}

type FuncContext interface {
	Context
}

// CallSiteLocProvider is implemented by FuncContext implementations that can
// return the location where the function was called from (the call site), as
// opposed to Loc() which returns the current execution position in the parent.
// This is used by generator backtrace capture to record the exact call site.
type CallSiteLocProvider interface {
	CallSiteLoc() *Loc
}

// ClosureStaticVarKeyProvider is an optional interface implemented by FuncContext
// when running inside a specific closure instance. The key is a uintptr that
// uniquely identifies the closure instance (typically its pointer address).
// This is used by runStaticVar to provide per-closure-instance static variable storage.
type ClosureStaticVarKeyProvider interface {
	ClosureStaticVarKey() uintptr
}

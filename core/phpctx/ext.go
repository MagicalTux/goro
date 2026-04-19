package phpctx

import (
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

var (
	globalExtMap         map[string]*Ext          = make(map[string]*Ext)
	globalFuncMap        map[string]phpv.Callable = make(map[string]phpv.Callable)
	globalConstantExtMap map[string]string        = make(map[string]string) // constant name → extension name
)

type Ext struct {
	Name           string
	Version        string
	Functions      map[string]*ExtFunction
	Constants      map[phpv.ZString]phpv.Val
	Classes        []*phpobj.ZClass
	// OnGlobalCreate is an optional callback invoked when a new Global context is created.
	// Extensions can use it to register per-Global resources such as stream handlers.
	OnGlobalCreate func(g *Global)
}

type ExtFunction struct {
	phpv.CallableVal
	name     string
	Ext      string // extension name, populated by RegisterExt
	Func     func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)
	Args     []*ExtFunctionArg
	MinArgs  int  // minimum required arguments (0 = no check)
	MaxArgs  int  // maximum allowed arguments (0 = no check, -1 = variadic/unlimited)
	ZeroArgs bool // if true, the function accepts exactly 0 arguments (MaxArgs=0 special case)
	funcArgs []*phpv.FuncArg // cached conversion of Args, populated at registration
}

func (e *ExtFunction) Name() string {
	return e.name
}

// IsBuiltinCallable implements phpv.BuiltinCallable, marking ExtFunction as a built-in.
func (e *ExtFunction) IsBuiltinCallable() {}

func (e *ExtFunction) GetExt() string {
	return e.Ext
}

func (e *ExtFunction) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// PHP 8: strict argument count checking for built-in functions
	if e.ZeroArgs && len(args) > 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s() expects exactly 0 arguments, %d given", e.name, len(args)))
	}
	if e.MaxArgs > 0 && len(args) > e.MaxArgs {
		if e.MinArgs == e.MaxArgs {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s() expects exactly %d argument, %d given", e.name, e.MaxArgs, len(args)))
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s() expects at most %d arguments, %d given", e.name, e.MaxArgs, len(args)))
	}
	if e.MinArgs > 0 && len(args) < e.MinArgs {
		if e.MinArgs == e.MaxArgs {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s() expects exactly %d argument, %d given", e.name, e.MinArgs, len(args)))
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s() expects at least %d arguments, %d given", e.name, e.MinArgs, len(args)))
	}
	return e.Func(ctx, args)
}

type ExtFunctionArg struct {
	ArgName   string // without the $ sign
	Ref       bool
	PreferRef bool      // silently accepts non-ref values (ZEND_SEND_PREFER_REF) — like extract()
	NoticeRef bool      // emits Notice for non-variable (ZEND_SEND_BY_REF) — like array_pop(), sort()
	Optional  bool      // is this argument optional?
	Variadic  bool      // is this a variadic parameter? (applies to all remaining args)
	Sensitive bool      // if true, this parameter is marked #[\SensitiveParameter] and masked in stack traces
}

// GetArgs implements phpv.FuncGetArgs, returning cached parameter metadata.
// Returns nil for functions without declared Args (most built-in functions),
// which signals to callZValImpl that Go-side argument handling applies.
func (e *ExtFunction) GetArgs() []*phpv.FuncArg {
	return e.funcArgs
}

// buildFuncArgs converts ExtFunctionArg metadata to FuncArg and caches it.
// Called once at registration time by RegisterExt.
func (e *ExtFunction) buildFuncArgs() {
	if len(e.Args) == 0 {
		return
	}
	e.funcArgs = make([]*phpv.FuncArg, len(e.Args))
	for i, a := range e.Args {
		e.funcArgs[i] = &phpv.FuncArg{
			VarName:   phpv.ZString(a.ArgName),
			Required:  !a.Optional,
			Ref:       a.Ref || a.PreferRef || a.NoticeRef,
			PreferRef: a.PreferRef,
			NoticeRef: a.NoticeRef,
			Variadic:  a.Variadic,
			Sensitive: a.Sensitive,
		}
	}
}

func RegisterExt(e *Ext) {
	globalExtMap[e.Name] = e
	for name, fn := range e.Functions {
		fn.name = name
		fn.Ext = e.Name
		fn.buildFuncArgs()
	}
	for _, class := range e.Classes {
		if class.Ext == "" {
			class.Ext = e.Name
		}
		for _, m := range class.Methods {
			if m.Class == nil {
				m.Class = class
			}
		}
	}
	// Track which extension owns each constant.
	for k := range e.Constants {
		globalConstantExtMap[string(k)] = e.Name
	}
}

// GetConstantExtName returns the name of the extension that defines the given
// constant, or "" if the constant was not registered by any extension.
func GetConstantExtName(name string) string {
	return globalConstantExtMap[name]
}

func HasExt(name string) bool {
	if _, res := globalExtMap[name]; res {
		return true
	}
	// Case-insensitive fallback
	lower := strings.ToLower(name)
	for k := range globalExtMap {
		if strings.ToLower(k) == lower {
			return true
		}
	}
	return false
}

func GetExt(name string) *Ext {
	if v, ok := globalExtMap[name]; ok {
		return v
	}
	// Case-insensitive fallback
	lower := strings.ToLower(name)
	for k, v := range globalExtMap {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return nil
}

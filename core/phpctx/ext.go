package phpctx

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var (
	globalExtMap  map[string]*Ext          = make(map[string]*Ext)
	globalFuncMap map[string]phpv.Callable = make(map[string]phpv.Callable)
)

type Ext struct {
	Name      string
	Version   string
	Functions map[string]*ExtFunction
	Constants map[phpv.ZString]phpv.Val
	Classes   []*phpobj.ZClass
}

type ExtFunction struct {
	phpv.CallableVal
	name     string
	Func     func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)
	Args     []*ExtFunctionArg
	MinArgs  int // minimum required arguments (0 = no check)
	MaxArgs  int // maximum allowed arguments (0 = no check, -1 = variadic/unlimited)
	funcArgs []*phpv.FuncArg // cached conversion of Args, populated at registration
}

func (e *ExtFunction) Name() string {
	return e.name
}

func (e *ExtFunction) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// PHP 8: strict argument count checking for built-in functions
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
	PreferRef bool // like Ref but silently accepts non-ref values (ZEND_SEND_PREFER_REF)
	Optional  bool // is this argument optional?
	Variadic  bool // is this a variadic parameter? (applies to all remaining args)
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
			Ref:       a.Ref || a.PreferRef,
			PreferRef: a.PreferRef,
			Variadic:  a.Variadic,
		}
	}
}

func RegisterExt(e *Ext) {
	globalExtMap[e.Name] = e
	for name, fn := range e.Functions {
		fn.name = name
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
}

func HasExt(name string) bool {
	_, res := globalExtMap[name]
	return res
}

func GetExt(name string) *Ext {
	v, ok := globalExtMap[name]
	if ok {
		return v
	}
	return nil
}

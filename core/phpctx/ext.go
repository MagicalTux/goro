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
	name    string
	Func    func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)
	Args    []*ExtFunctionArg
	MinArgs int // minimum required arguments (0 = no check)
	MaxArgs int // maximum allowed arguments (0 = no check, -1 = variadic/unlimited)
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
	ArgName  string // without the $ sign
	Ref      bool
	Optional bool // is this argument optional?
}

// GetArgs implements phpv.FuncGetArgs so that closureFromCallable can
// read parameter metadata for the closure's __debugInfo output.
func (e *ExtFunction) GetArgs() []*phpv.FuncArg {
	if len(e.Args) == 0 {
		return nil
	}
	args := make([]*phpv.FuncArg, len(e.Args))
	for i, a := range e.Args {
		args[i] = &phpv.FuncArg{
			VarName:  phpv.ZString(a.ArgName),
			Required: !a.Optional,
			Ref:      a.Ref,
		}
	}
	return args
}

func RegisterExt(e *Ext) {
	globalExtMap[e.Name] = e
	for name, fn := range e.Functions {
		fn.name = name
	}
	for _, class := range e.Classes {
		for _, m := range class.Methods {
			m.Class = class
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

package phpctx

import "github.com/MagicalTux/goro/core/phpv"

var (
	globalExtMap  map[string]*Ext          = make(map[string]*Ext)
	globalFuncMap map[string]phpv.Callable = make(map[string]phpv.Callable)
)

type Ext struct {
	Name      string
	Version   string
	Functions map[string]*ExtFunction
	Constants map[phpv.ZString]phpv.Val
	Classes   []phpv.ZClass
}

type ExtFunction struct {
	name string
	Func func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)
	Args []*ExtFunctionArg
}

func (e *ExtFunction) Name() string {
	return e.name
}

func (e *ExtFunction) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return e.Func(ctx, args)
}

func (e *ExtFunction) GetArgs() []*phpv.FuncArg {
	var args []*phpv.FuncArg
	for _, arg := range e.Args {
		args = append(args, &phpv.FuncArg{
			VarName:  phpv.ZString(arg.ArgName),
			Ref:      arg.Ref,
			Required: !arg.Optional,
		})
	}
	return args
}

type ExtFunctionArg struct {
	ArgName  string // without the $ sign
	Ref      bool
	Optional bool // is this argument optional?
}

func RegisterExt(e *Ext) {
	globalExtMap[e.Name] = e
	for name, fn := range e.Functions {
		fn.name = name
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

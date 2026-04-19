package reflection

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// ReflectionExtension class - handles extension reflection.
var ReflectionExtension *phpobj.ZClass

func initReflectionExtension() {
	ReflectionExtension = &phpobj.ZClass{
		Name: "ReflectionExtension",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionExtension::__construct() expects exactly 1 argument, 0 given")
				}
				if len(args) > 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionExtension::__construct() expects exactly 1 argument, %d given", len(args)))
				}
				if args[0].GetType() == phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionExtension::__construct(): Argument #1 ($name) must be of type string, array given")
				}
				name := args[0].AsString(ctx)
				o.HashTable().SetString("name", name.ZVal())
				o.SetOpaque(ReflectionExtension, name)
				return nil, nil
			})},
			"getname": {Name: "getName", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				if ext != nil {
					return phpv.ZString(ext.Name).ZVal(), nil
				}
				return name.ZVal(), nil
			})},
			"getversion": {Name: "getVersion", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				if ext != nil && ext.Version != "" {
					return phpv.ZString(ext.Version).ZVal(), nil
				}
				return phpv.ZBool(false).ZVal(), nil
			})},
			"getfunctions": {Name: "getFunctions", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				arr := phpv.NewZArray()
				if ext == nil {
					return arr.ZVal(), nil
				}
				for fnName := range ext.Functions {
					fn, err := ctx.Global().GetFunction(ctx, phpv.ZString(fnName))
					if err != nil {
						continue
					}
					rfObj, err2 := phpobj.CreateZObject(ctx, ReflectionFunction)
					if err2 != nil {
						continue
					}
					data := &reflectionFunctionData{
						name:     phpv.ZString(fnName),
						callable: fn,
					}
					if fga, ok := fn.(phpv.FuncGetArgs); ok {
						data.args = fga.GetArgs()
					}
					rfObj.HashTable().SetString("name", phpv.ZString(fnName).ZVal())
					rfObj.SetOpaque(ReflectionFunction, data)
					arr.OffsetSet(ctx, phpv.ZString(fnName).ZVal(), rfObj.ZVal())
				}
				return arr.ZVal(), nil
			})},
			"getconstants": {Name: "getConstants", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				arr := phpv.NewZArray()
				if ext == nil {
					return arr.ZVal(), nil
				}
				for constName, constVal := range ext.Constants {
					arr.OffsetSet(ctx, constName.ZVal(), constVal.ZVal())
				}
				return arr.ZVal(), nil
			})},
			"getclasses": {Name: "getClasses", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				arr := phpv.NewZArray()
				if ext == nil {
					return arr.ZVal(), nil
				}
				for _, class := range ext.Classes {
					rcObj, err := phpobj.CreateZObject(ctx, ReflectionClass)
					if err != nil {
						continue
					}
					className := class.GetName()
					rcObj.HashTable().SetString("name", className.ZVal())
					rcObj.SetOpaque(ReflectionClass, phpv.ZClass(class))
					arr.OffsetSet(ctx, className.ZVal(), rcObj.ZVal())
				}
				return arr.ZVal(), nil
			})},
			"getclassnames": {Name: "getClassNames", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				arr := phpv.NewZArray()
				if ext == nil {
					return arr.ZVal(), nil
				}
				for _, class := range ext.Classes {
					arr.OffsetSet(ctx, nil, class.GetName().ZVal())
				}
				return arr.ZVal(), nil
			})},
			"getinientries": {Name: "getINIEntries", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Return INI entries - for now return empty since we don't have per-ext INI
				return phpv.NewZArray().ZVal(), nil
			})},
			"info": {Name: "info", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				// Use the canonical extension name from the registry
				ext := phpctx.GetExt(string(name))
				extName := string(name)
				if ext != nil {
					extName = ext.Name
				}
				// Print extension info like PHP does
				fmt.Fprintf(ctx, "%s\n\n%s => enabled\n", extName, extName)
				return phpv.ZNULL.ZVal(), nil
			})},
			"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				ext := phpctx.GetExt(string(name))
				version := ""
				if ext != nil {
					version = ext.Version
				}
				return phpv.ZString(fmt.Sprintf("Extension [ <persistent> extension #0 %s version %s ] {\n}\n", name, version)).ZVal(), nil
			})},
			"ispersistent": {Name: "isPersistent", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZBool(true).ZVal(), nil
			})},
			"istemporary": {Name: "isTemporary", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZBool(false).ZVal(), nil
			})},
			"getdependencies": {Name: "getDependencies", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
		},
	}
}

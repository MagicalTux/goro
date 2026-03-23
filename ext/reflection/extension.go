package reflection

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionExtension class - a stub that handles basic extension reflection.
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
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionExtension::__construct() expects exactly 1 argument, 0 given")
				}
				name := args[0].AsString(ctx)
				o.HashTable().SetString("name", name.ZVal())
				o.SetOpaque(ReflectionExtension, name)
				return nil, nil
			})},
			"getname": {Name: "getName", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				return name.ZVal(), nil
			})},
			"getversion": {Name: "getVersion", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZBool(false).ZVal(), nil
			})},
			"getfunctions": {Name: "getFunctions", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"getconstants": {Name: "getConstants", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"getclasses": {Name: "getClasses", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"getclassnames": {Name: "getClassNames", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"getinientries": {Name: "getINIEntries", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"info": {Name: "info", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZNULL.ZVal(), nil
			})},
			"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				name, _ := o.GetOpaque(ReflectionExtension).(phpv.ZString)
				return phpv.ZString("Extension [ <persistent> extension #0 " + string(name) + " version  ] {\n}\n").ZVal(), nil
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

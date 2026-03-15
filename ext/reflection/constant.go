package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// reflectionConstantData is stored as opaque data on ReflectionConstant objects
type reflectionConstantData struct {
	name  phpv.ZString
	value phpv.Val
}

func initReflectionConstant() {
	ReflectionConstant.Props = []*phpv.ZClassProp{
		{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
	}
	ReflectionConstant.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":   {Name: "__construct", Method: phpobj.NativeMethod(reflectionConstantConstruct)},
		"getname":       {Name: "getName", Method: phpobj.NativeMethod(reflectionConstantGetName)},
		"getvalue":      {Name: "getValue", Method: phpobj.NativeMethod(reflectionConstantGetValue)},
		"getattributes": {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionConstantGetAttributes)},
		"__tostring":    {Name: "__toString", Method: phpobj.NativeMethod(reflectionConstantToString)},
	}
}

func reflectionConstantConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionConstant::__construct() expects exactly 1 argument, 0 given")
	}

	name := args[0].AsString(ctx)

	// Look up the global constant
	g := ctx.Global()
	val, ok := g.ConstantGet(name)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Constant \"%s\" does not exist", name))
	}

	data := &reflectionConstantData{
		name:  name,
		value: val,
	}
	o.HashTable().SetString("name", name.ZVal())
	o.SetOpaque(ReflectionConstant, data)
	return nil, nil
}

func getConstData(o *phpobj.ZObject) *reflectionConstantData {
	v := o.GetOpaque(ReflectionConstant)
	if v == nil {
		return nil
	}
	return v.(*reflectionConstantData)
}

func reflectionConstantGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.name.ZVal(), nil
}

func reflectionConstantGetValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if data.value == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return data.value.ZVal(), nil
}

func reflectionConstantGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	attrs := ctx.Global().ConstantGetAttributes(data.name)
	if len(attrs) == 0 {
		return phpv.NewZArray().ZVal(), nil
	}
	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, attrs, phpobj.AttributeTARGET_CONSTANT, name, flags)
}

func reflectionConstantToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.ZString("Constant [ ]").ZVal(), nil
	}
	return phpv.ZString(fmt.Sprintf("Constant [ %s ]", data.name)).ZVal(), nil
}

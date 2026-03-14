package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionFunction class

var ReflectionFunction *phpobj.ZClass

// reflectionFunctionData is stored as opaque data on ReflectionFunction objects
type reflectionFunctionData struct {
	name     phpv.ZString
	callable phpv.Callable
	args     []*phpv.FuncArg // may be nil if callable doesn't implement FuncGetArgs
}

func initReflectionFunction() {
	ReflectionFunction = &phpobj.ZClass{
		Name: "ReflectionFunction",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":                {Name: "__construct", Method: phpobj.NativeMethod(reflectionFunctionConstruct)},
			"getname":                    {Name: "getName", Method: phpobj.NativeMethod(reflectionFunctionGetName)},
			"getnumberofparameters":      {Name: "getNumberOfParameters", Method: phpobj.NativeMethod(reflectionFunctionGetNumberOfParameters)},
			"getnumberofrequiredparameters": {Name: "getNumberOfRequiredParameters", Method: phpobj.NativeMethod(reflectionFunctionGetNumberOfRequiredParameters)},
			"getparameters":              {Name: "getParameters", Method: phpobj.NativeMethod(reflectionFunctionGetParameters)},
			"invoke":                     {Name: "invoke", Method: phpobj.NativeMethod(reflectionFunctionInvoke)},
			"getattributes":              {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionFunctionGetAttributes)},
		},
	}
}

func reflectionFunctionConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionFunction::__construct() expects exactly 1 argument, 0 given")
	}

	arg := args[0]
	data := &reflectionFunctionData{}

	if arg.GetType() == phpv.ZtString {
		// Function name
		funcName := arg.AsString(ctx)
		fn, err := ctx.Global().GetFunction(ctx, funcName)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Function %s() does not exist", funcName))
		}
		data.name = funcName
		data.callable = fn
		if fga, ok := fn.(phpv.FuncGetArgs); ok {
			data.args = fga.GetArgs()
		}
	} else if arg.GetType() == phpv.ZtObject {
		// Closure object
		obj := arg.AsObject(ctx)
		if obj != nil {
			opaque := obj.GetOpaque(obj.GetClass())
			if closure, ok := opaque.(phpv.ZClosure); ok {
				data.name = phpv.ZString(closure.Name())
				data.callable = closure
				data.args = closure.GetArgs()
			} else {
				return nil, phpobj.ThrowError(ctx, ReflectionException, "Function() does not exist")
			}
		}
	} else {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Function() does not exist")
	}

	o.HashTable().SetString("name", data.name.ZVal())
	o.SetOpaque(ReflectionFunction, data)
	return nil, nil
}

func getFuncData(o *phpobj.ZObject) *reflectionFunctionData {
	v := o.GetOpaque(ReflectionFunction)
	if v == nil {
		return nil
	}
	return v.(*reflectionFunctionData)
}

func reflectionFunctionGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.name.ZVal(), nil
}

func reflectionFunctionGetNumberOfParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.args == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(len(data.args)).ZVal(), nil
}

func reflectionFunctionGetNumberOfRequiredParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.args == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	count := 0
	for _, a := range data.args {
		if a.Required {
			count++
		}
	}
	return phpv.ZInt(count).ZVal(), nil
}

func reflectionFunctionGetParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.args == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	return createReflectionParameterObjects(ctx, data.args, data.name)
}

func reflectionFunctionInvoke(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.callable == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}
	return ctx.CallZVal(ctx, data.callable, args)
}

func reflectionFunctionGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Functions don't have attributes stored yet - return empty array
	return phpv.NewZArray().ZVal(), nil
}

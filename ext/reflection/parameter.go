package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionParameter class

var ReflectionParameter *phpobj.ZClass

// reflectionParameterData is stored as opaque data on ReflectionParameter objects
type reflectionParameterData struct {
	arg      *phpv.FuncArg
	position int
	funcName phpv.ZString
}

func initReflectionParameter() {
	ReflectionParameter = &phpobj.ZClass{
		Name: "ReflectionParameter",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":         {Name: "__construct", Method: phpobj.NativeMethod(reflectionParameterConstruct)},
			"getname":             {Name: "getName", Method: phpobj.NativeMethod(reflectionParameterGetName)},
			"getposition":         {Name: "getPosition", Method: phpobj.NativeMethod(reflectionParameterGetPosition)},
			"isoptional":          {Name: "isOptional", Method: phpobj.NativeMethod(reflectionParameterIsOptional)},
			"hasdefaultvalue":     {Name: "hasDefaultValue", Method: phpobj.NativeMethod(reflectionParameterHasDefaultValue)},
			"getdefaultvalue":     {Name: "getDefaultValue", Method: phpobj.NativeMethod(reflectionParameterGetDefaultValue)},
			"gettype":             {Name: "getType", Method: phpobj.NativeMethod(reflectionParameterGetType)},
			"allowsnull":          {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionParameterAllowsNull)},
			"ispassedbyreference": {Name: "isPassedByReference", Method: phpobj.NativeMethod(reflectionParameterIsPassedByReference)},
			"isvariadic":          {Name: "isVariadic", Method: phpobj.NativeMethod(reflectionParameterIsVariadic)},
		},
	}
}

func reflectionParameterConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionParameter::__construct() expects exactly 2 arguments")
	}

	// Get the function/method's args
	var funcArgs []*phpv.FuncArg
	var funcName phpv.ZString

	funcVal := args[0]
	if funcVal.GetType() == phpv.ZtString {
		// Function name
		funcName = funcVal.AsString(ctx)
		fn, err := ctx.Global().GetFunction(ctx, funcName)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Function %s() does not exist", funcName))
		}
		if fga, ok := fn.(phpv.FuncGetArgs); ok {
			funcArgs = fga.GetArgs()
		}
	} else if funcVal.GetType() == phpv.ZtArray {
		// [class, method] callable
		return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionParameter::__construct(): array callables not yet supported")
	} else if funcVal.GetType() == phpv.ZtObject {
		// Closure
		obj := funcVal.AsObject(ctx)
		if obj != nil {
			opaque := obj.GetOpaque(obj.GetClass())
			if closure, ok := opaque.(phpv.FuncGetArgs); ok {
				funcArgs = closure.GetArgs()
				funcName = phpv.ZString(opaque.(phpv.Callable).Name())
			}
		}
	}

	param := args[1]
	var paramData *reflectionParameterData

	if param.GetType() == phpv.ZtInt {
		// Position-based lookup
		pos := int(param.AsInt(ctx))
		if funcArgs == nil || pos < 0 || pos >= len(funcArgs) {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("The parameter specified by its index does not exist"))
		}
		paramData = &reflectionParameterData{
			arg:      funcArgs[pos],
			position: pos,
			funcName: funcName,
		}
	} else {
		// Name-based lookup
		paramName := param.AsString(ctx)
		if funcArgs != nil {
			for i, a := range funcArgs {
				if a.VarName == paramName {
					paramData = &reflectionParameterData{
						arg:      a,
						position: i,
						funcName: funcName,
					}
					break
				}
			}
		}
		if paramData == nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("The parameter specified by its name could not be found"))
		}
	}

	o.HashTable().SetString("name", paramData.arg.VarName.ZVal())
	o.SetOpaque(ReflectionParameter, paramData)
	return nil, nil
}

func getParamData(o *phpobj.ZObject) *reflectionParameterData {
	v := o.GetOpaque(ReflectionParameter)
	if v == nil {
		return nil
	}
	return v.(*reflectionParameterData)
}

func reflectionParameterGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.arg.VarName.ZVal(), nil
}

func reflectionParameterGetPosition(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(data.position).ZVal(), nil
}

func reflectionParameterIsOptional(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(!data.arg.Required).ZVal(), nil
}

func reflectionParameterHasDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.arg.DefaultValue != nil).ZVal(), nil
}

func reflectionParameterGetDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.DefaultValue == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the default value")
	}
	return data.arg.DefaultValue.ZVal(), nil
}

func reflectionParameterGetType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.Hint == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionTypeObject(ctx, data.arg.Hint)
}

func reflectionParameterAllowsNull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(true).ZVal(), nil
	}
	// If no type hint, null is allowed
	if data.arg.Hint == nil {
		return phpv.ZBool(true).ZVal(), nil
	}
	// If nullable type hint or implicitly nullable (type + null default)
	if data.arg.Hint.Nullable || data.arg.ImplicitlyNullable {
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionParameterIsPassedByReference(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.arg.Ref).ZVal(), nil
}

func reflectionParameterIsVariadic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.arg.Variadic).ZVal(), nil
}

// createReflectionParameterObjects creates an array of ReflectionParameter objects
// from a slice of FuncArg.
func createReflectionParameterObjects(ctx phpv.Context, funcArgs []*phpv.FuncArg, funcName phpv.ZString) (*phpv.ZVal, error) {
	arr := phpv.NewZArray()
	for i, arg := range funcArgs {
		obj, err := phpobj.CreateZObject(ctx, ReflectionParameter)
		if err != nil {
			return nil, err
		}
		data := &reflectionParameterData{
			arg:      arg,
			position: i,
			funcName: funcName,
		}
		obj.HashTable().SetString("name", arg.VarName.ZVal())
		obj.SetOpaque(ReflectionParameter, data)
		arr.OffsetSet(ctx, nil, obj.ZVal())
	}
	return arr.ZVal(), nil
}

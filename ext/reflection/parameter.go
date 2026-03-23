package reflection

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
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
			"getattributes":       {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionParameterGetAttributes)},
			"hastype":                  {Name: "hasType", Method: phpobj.NativeMethod(reflectionParameterHasType)},
			"__tostring":               {Name: "__toString", Method: phpobj.NativeMethod(reflectionParameterToString)},
			"isdefaultvalueavailable":      {Name: "isDefaultValueAvailable", Method: phpobj.NativeMethod(reflectionParameterIsDefaultValueAvailable)},
			"getdeclaringfunction":         {Name: "getDeclaringFunction", Method: phpobj.NativeMethod(reflectionParameterGetDeclaringFunction)},
			"getdeclaringclass":            {Name: "getDeclaringClass", Method: phpobj.NativeMethod(reflectionParameterGetDeclaringClass)},
			"ispromoted":                   {Name: "isPromoted", Method: phpobj.NativeMethod(reflectionParameterIsPromoted)},
			"isdefaultvalueconstant":       {Name: "isDefaultValueConstant", Method: phpobj.NativeMethod(reflectionParameterIsDefaultValueConstant)},
			"getdefaultvalueconstantname":  {Name: "getDefaultValueConstantName", Method: phpobj.NativeMethod(reflectionParameterGetDefaultValueConstantName)},
			"canbepassedbyvalue":           {Name: "canBePassedByValue", Method: phpobj.NativeMethod(reflectionParameterCanBePassedByValue)},
			"isarray":                     {Name: "isArray", Method: phpobj.NativeMethod(reflectionParameterIsArray)},
			"iscallable":                  {Name: "isCallable", Method: phpobj.NativeMethod(reflectionParameterIsCallable)},
			"getclass":                    {Name: "getClass", Method: phpobj.NativeMethod(reflectionParameterGetClass)},
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
		arr := funcVal.AsArray(ctx)
		if arr == nil || arr.Count(ctx) != 2 {
			return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionParameter::__construct(): Expected array with class and method name")
		}
		classVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
		methodVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
		if classVal == nil || methodVal == nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionParameter::__construct(): Expected array with class and method name")
		}
		className := classVal.AsString(ctx)
		methodName := methodVal.AsString(ctx)
		class, err := ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" does not exist", className))
		}
		method, ok := class.GetMethod(methodName)
		if !ok {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s() does not exist", className, methodName))
		}
		if fga, ok2 := method.Method.(phpv.FuncGetArgs); ok2 {
			funcArgs = fga.GetArgs()
		}
		funcName = phpv.ZString(fmt.Sprintf("%s::%s", className, methodName))
	} else if funcVal.GetType() == phpv.ZtObject {
		// Closure
		obj := funcVal.AsObject(ctx)
		if obj != nil {
			opaque := obj.GetOpaque(obj.GetClass())
			if closure, ok := opaque.(phpv.ZClosure); ok {
				funcArgs = closure.GetArgs()
				funcName = phpv.ZString(closure.Name())
			}
		}
	}

	param := args[1]
	var paramData *reflectionParameterData

	if param.GetType() == phpv.ZtInt {
		// Position-based lookup
		pos := int(param.AsInt(ctx))
		if pos < 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionParameter::__construct(): Argument #2 ($param) must be greater than or equal to 0")
		}
		if funcArgs == nil || pos >= len(funcArgs) {
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
	// DefaultValue != nil means the parameter has a default
	// This covers both regular values and CompileDelayed values
	return phpv.ZBool(data.arg.DefaultValue != nil).ZVal(), nil
}

func reflectionParameterGetDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.DefaultValue == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the default value")
	}
	// Resolve CompileDelayed values
	if cd, ok := data.arg.DefaultValue.(*phpv.CompileDelayed); ok {
		resolved, err := cd.Run(ctx)
		if err != nil {
			return nil, err
		}
		return resolved, nil
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

func reflectionParameterHasType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.Hint == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
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

func reflectionParameterGetDeclaringClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if strings.Contains(string(data.funcName), "::") {
		parts := strings.SplitN(string(data.funcName), "::", 2)
		class, err := ctx.Global().GetClass(ctx, phpv.ZString(parts[0]), false)
		if err == nil {
			return createReflectionClassObject(ctx, class)
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

func reflectionParameterIsPromoted(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.arg.Promotion != 0).ZVal(), nil
}

func reflectionParameterIsDefaultValueConstant(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.DefaultValue == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the default value")
	}
	// We don't track whether a default value came from a constant expression,
	// so return false for now
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionParameterGetDefaultValueConstantName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil || data.arg.DefaultValue == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the default value")
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionParameterCanBePassedByValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(true).ZVal(), nil
	}
	// canBePassedByValue returns true if the parameter is NOT a reference parameter
	return phpv.ZBool(!data.arg.Ref).ZVal(), nil
}

func reflectionParameterIsArray(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_ = ctx.Deprecated("Method ReflectionParameter::isArray() is deprecated since 8.0, use ReflectionParameter::getType() instead", logopt.NoFuncName(true))
	data := getParamData(o)
	if data == nil || data.arg.Hint == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Check if the type hint is exactly "array" (not a union type containing array)
	if data.arg.Hint.Type() == phpv.ZtArray && len(data.arg.Hint.Union) == 0 {
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionParameterIsCallable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_ = ctx.Deprecated("Method ReflectionParameter::isCallable() is deprecated since 8.0, use ReflectionParameter::getType() instead", logopt.NoFuncName(true))
	data := getParamData(o)
	if data == nil || data.arg.Hint == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.arg.Hint.ClassName() == "callable" && len(data.arg.Hint.Union) == 0 {
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionParameterGetClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	_ = ctx.Deprecated("Method ReflectionParameter::getClass() is deprecated since 8.0, use ReflectionParameter::getType() instead", logopt.NoFuncName(true))
	data := getParamData(o)
	if data == nil || data.arg.Hint == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// Only return a class for class/interface type hints (not built-in types)
	className := data.arg.Hint.ClassName()
	if className == "" || className == "callable" || className == "iterable" {
		return phpv.ZNULL.ZVal(), nil
	}
	// Check if it's a built-in type
	switch data.arg.Hint.Type() {
	case phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat, phpv.ZtString, phpv.ZtArray, phpv.ZtNull, phpv.ZtVoid, phpv.ZtNever, phpv.ZtMixed:
		return phpv.ZNULL.ZVal(), nil
	}
	// Try to resolve the class
	class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionClassObject(ctx, class)
}

func reflectionParameterGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.arg.Attributes, phpobj.AttributeTARGET_PARAMETER, name, flags)
}

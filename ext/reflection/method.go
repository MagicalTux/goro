package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// reflectionMethodData is stored as opaque data on ReflectionMethod objects
type reflectionMethodData struct {
	method *phpv.ZClassMethod
	class  phpv.ZClass
}

func initReflectionMethod() {
	// ReflectionMethod is declared in ext.go; we add methods here
	ReflectionMethod.Props = []*phpv.ZClassProp{
		{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		{VarName: "class", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
	}
	ReflectionMethod.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":                   {Name: "__construct", Method: phpobj.NativeMethod(reflectionMethodConstructFull)},
		"getname":                       {Name: "getName", Method: phpobj.NativeMethod(reflectionMethodGetName)},
		"getdeclaringclass":             {Name: "getDeclaringClass", Method: phpobj.NativeMethod(reflectionMethodGetDeclaringClass)},
		"ispublic":                      {Name: "isPublic", Method: phpobj.NativeMethod(reflectionMethodIsPublic)},
		"isprotected":                   {Name: "isProtected", Method: phpobj.NativeMethod(reflectionMethodIsProtected)},
		"isprivate":                     {Name: "isPrivate", Method: phpobj.NativeMethod(reflectionMethodIsPrivate)},
		"isstatic":                      {Name: "isStatic", Method: phpobj.NativeMethod(reflectionMethodIsStatic)},
		"isabstract":                    {Name: "isAbstract", Method: phpobj.NativeMethod(reflectionMethodIsAbstract)},
		"isfinal":                       {Name: "isFinal", Method: phpobj.NativeMethod(reflectionMethodIsFinal)},
		"isconstructor":                 {Name: "isConstructor", Method: phpobj.NativeMethod(reflectionMethodIsConstructor)},
		"getnumberofparameters":         {Name: "getNumberOfParameters", Method: phpobj.NativeMethod(reflectionMethodGetNumberOfParameters)},
		"getnumberofrequiredparameters": {Name: "getNumberOfRequiredParameters", Method: phpobj.NativeMethod(reflectionMethodGetNumberOfRequiredParameters)},
		"getparameters":                 {Name: "getParameters", Method: phpobj.NativeMethod(reflectionMethodGetParameters)},
		"invoke":                        {Name: "invoke", Method: phpobj.NativeMethod(reflectionMethodInvoke)},
		"getattributes":                 {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionMethodGetAttributes)},
		"getclosure":                    {Name: "getClosure", Method: phpobj.NativeMethod(reflectionMethodGetClosure)},
	}
}

func reflectionMethodConstructFull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::__construct() expects exactly 2 arguments")
	}

	var class phpv.ZClass
	var err error

	// First arg can be object or class name
	if args[0].GetType() == phpv.ZtObject {
		class = args[0].AsObject(ctx).GetClass()
	} else {
		className := args[0].AsString(ctx)
		class, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	}

	methodName := args[1].AsString(ctx)
	method, ok := class.GetMethod(methodName)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s() does not exist", class.GetName(), methodName))
	}

	data := &reflectionMethodData{
		method: method,
		class:  class,
	}
	o.HashTable().SetString("name", method.Name.ZVal())
	o.HashTable().SetString("class", class.GetName().ZVal())
	o.SetOpaque(ReflectionMethod, data)
	return nil, nil
}

func getMethodData(o *phpobj.ZObject) *reflectionMethodData {
	v := o.GetOpaque(ReflectionMethod)
	if v == nil {
		return nil
	}
	return v.(*reflectionMethodData)
}

func reflectionMethodGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.method.Name.ZVal(), nil
}

func reflectionMethodGetDeclaringClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// The declaring class is the class where this method is actually defined
	declaringClass := data.class
	if data.method.Class != nil {
		declaringClass = data.method.Class
	}

	return createReflectionClassObject(ctx, declaringClass)
}

func reflectionMethodIsPublic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// If no access modifier is set, method is implicitly public
	access := data.method.Modifiers.Access()
	isPublic := access == phpv.ZAttrPublic || access == 0 || data.method.Modifiers.Has(phpv.ZAttrImplicitPublic)
	return phpv.ZBool(isPublic).ZVal(), nil
}

func reflectionMethodIsProtected(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Modifiers.IsProtected()).ZVal(), nil
}

func reflectionMethodIsPrivate(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Modifiers.IsPrivate()).ZVal(), nil
}

func reflectionMethodIsStatic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Modifiers.IsStatic()).ZVal(), nil
}

func reflectionMethodIsAbstract(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Modifiers.Has(phpv.ZAttrAbstract) || data.method.Empty).ZVal(), nil
}

func reflectionMethodIsFinal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Modifiers.Has(phpv.ZAttrFinal)).ZVal(), nil
}

func reflectionMethodIsConstructor(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Name.ToLower() == "__construct").ZVal(), nil
}

func reflectionMethodGetNumberOfParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		return phpv.ZInt(len(fga.GetArgs())).ZVal(), nil
	}
	return phpv.ZInt(0).ZVal(), nil
}

func reflectionMethodGetNumberOfRequiredParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		count := 0
		for _, a := range fga.GetArgs() {
			if a.Required {
				count++
			}
		}
		return phpv.ZInt(count).ZVal(), nil
	}
	return phpv.ZInt(0).ZVal(), nil
}

func reflectionMethodGetParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		funcName := phpv.ZString(string(data.class.GetName()) + "::" + string(data.method.Name))
		return createReflectionParameterObjects(ctx, fga.GetArgs(), funcName)
	}
	return phpv.NewZArray().ZVal(), nil
}

func reflectionMethodInvoke(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::invoke() expects at least 1 argument")
	}

	// First argument is the object instance (or null for static methods)
	objArg := args[0]
	methodArgs := args[1:]

	if data.method.Modifiers.IsStatic() {
		// For static methods, call without $this
		return ctx.CallZVal(ctx, data.method.Method, methodArgs)
	}

	if objArg.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke non static method %s::%s() without an object", data.class.GetName(), data.method.Name))
	}

	obj := objArg.AsObject(ctx)
	return ctx.CallZVal(ctx, data.method.Method, methodArgs, obj)
}

// createReflectionMethodObject creates a ReflectionMethod object for the given
// class and method, without going through __construct.
func createReflectionMethodObject(ctx phpv.Context, class phpv.ZClass, method *phpv.ZClassMethod) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionMethod)
	if err != nil {
		return nil, err
	}
	data := &reflectionMethodData{
		method: method,
		class:  class,
	}
	obj.HashTable().SetString("name", method.Name.ZVal())
	obj.HashTable().SetString("class", class.GetName().ZVal())
	obj.SetOpaque(ReflectionMethod, data)
	return obj.ZVal(), nil
}

func reflectionMethodGetClosure(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Optional first arg: object instance
	var instance phpv.ZObject
	if len(args) > 0 && args[0].GetType() == phpv.ZtObject {
		instance = args[0].AsObject(ctx)
	}

	callable := data.method.Method
	if instance != nil {
		callable = phpv.Bind(callable, instance)
	}
	return phpv.NewZVal(callable), nil
}

func reflectionMethodGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.method.Attributes, phpobj.AttributeTARGET_METHOD, name, flags)
}

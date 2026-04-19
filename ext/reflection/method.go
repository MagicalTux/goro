package reflection

import (
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// reflectionMethodData is stored as opaque data on ReflectionMethod objects
type reflectionMethodData struct {
	method          *phpv.ZClassMethod
	class           phpv.ZClass
	closureCallable phpv.FuncGetArgs // non-nil when reflecting __invoke on a Closure instance
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
		"invokeargs":                    {Name: "invokeArgs", Method: phpobj.NativeMethod(reflectionMethodInvokeArgs)},
		"getattributes":                 {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionMethodGetAttributes)},
		"getclosure":                    {Name: "getClosure", Method: phpobj.NativeMethod(reflectionMethodGetClosure)},
		"getdoccomment":                 {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionMethodGetDocComment)},
		"isdeprecated":                  {Name: "isDeprecated", Method: phpobj.NativeMethod(reflectionMethodIsDeprecated)},
		"getreturntype":                 {Name: "getReturnType", Method: phpobj.NativeMethod(reflectionMethodGetReturnType)},
		"hasreturntype":                 {Name: "hasReturnType", Method: phpobj.NativeMethod(reflectionMethodHasReturnType)},
		"hastentativereturntype":        {Name: "hasTentativeReturnType", Method: phpobj.NativeMethod(reflectionMethodHasTentativeReturnType)},
		"gettentativereturntype":        {Name: "getTentativeReturnType", Method: phpobj.NativeMethod(reflectionMethodGetTentativeReturnType)},
		"hasprototype":                  {Name: "hasPrototype", Method: phpobj.NativeMethod(reflectionMethodHasPrototype)},
		"getprototype":                  {Name: "getPrototype", Method: phpobj.NativeMethod(reflectionMethodGetPrototype)},
		"isdestructor":                  {Name: "isDestructor", Method: phpobj.NativeMethod(reflectionMethodIsDestructor)},
		"isinternal":                    {Name: "isInternal", Method: phpobj.NativeMethod(reflectionMethodIsInternal)},
		"isuserdefined":                 {Name: "isUserDefined", Method: phpobj.NativeMethod(reflectionMethodIsUserDefined)},
		"getmodifiers":                  {Name: "getModifiers", Method: phpobj.NativeMethod(reflectionMethodGetModifiers)},
		"getfilename":                   {Name: "getFileName", Method: phpobj.NativeMethod(reflectionMethodGetFileName)},
		"getstartline":                  {Name: "getStartLine", Method: phpobj.NativeMethod(reflectionMethodGetStartLine)},
		"getendline":                    {Name: "getEndLine", Method: phpobj.NativeMethod(reflectionMethodGetEndLine)},
		"returnsreference":              {Name: "returnsReference", Method: phpobj.NativeMethod(reflectionMethodReturnsReference)},
		"isvariadic":                    {Name: "isVariadic", Method: phpobj.NativeMethod(reflectionMethodIsVariadic)},
		"getstaticvariables":            {Name: "getStaticVariables", Method: phpobj.NativeMethod(reflectionMethodGetStaticVariables)},
		"getextensionname":              {Name: "getExtensionName", Method: phpobj.NativeMethod(reflectionMethodGetExtensionName)},
		"setaccessible":                 {Name: "setAccessible", Method: phpobj.NativeMethod(reflectionMethodSetAccessible)},
		"__tostring":                    {Name: "__toString", Method: phpobj.NativeMethod(reflectionMethodToString)},
		"createfrommethodname":          {Name: "createFromMethodName", Method: phpobj.NativeMethod(reflectionMethodCreateFromMethodName), Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic},
		"isgenerator":                   {Name: "isGenerator", Method: phpobj.NativeMethod(reflectionMethodIsGenerator)},
		"getshortname":                  {Name: "getShortName", Method: phpobj.NativeMethod(reflectionMethodGetShortName)},
		"getnamespacename":              {Name: "getNamespaceName", Method: phpobj.NativeMethod(reflectionMethodGetNamespaceName)},
		"innamespace":                   {Name: "inNamespace", Method: phpobj.NativeMethod(reflectionMethodInNamespace)},
	}
}

// reflectionMethodGetDocComment returns the doc comment for a method.
func reflectionMethodGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data, ok := o.GetOpaque(ReflectionMethod).(*reflectionMethodData)
	if !ok || data == nil || data.method == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if data.method.DocComment == "" {
		return phpv.ZFalse.ZVal(), nil
	}
	return data.method.DocComment.ZVal(), nil
}

func reflectionMethodConstructFull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "ReflectionMethod::__construct() expects at least 1 argument, 0 given")
	}
	if len(args) > 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("ReflectionMethod::__construct() expects at most 2 arguments, %d given", len(args)))
	}

	var class phpv.ZClass
	var methodName phpv.ZString
	var err error

	if len(args) == 1 {
		// Single argument form: "ClassName::methodName"
		// Emit deprecation notice first (before validation)
		_ = ctx.Deprecated("Calling ReflectionMethod::__construct() with 1 argument is deprecated, use ReflectionMethod::createFromMethodName() instead", logopt.NoFuncName(true))

		methodStr := string(args[0].AsString(ctx))
		parts := strings.SplitN(methodStr, "::", 2)
		if len(parts) != 2 {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				fmt.Sprintf("ReflectionMethod::__construct(): Argument #1 ($objectOrMethod) must be a valid method name"))
		}

		className := phpv.ZString(parts[0])
		methodName = phpv.ZString(parts[1])

		class, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	} else {
		// Two argument form: (class/object, methodName)
		if args[0].GetType() == phpv.ZtObject {
			class = args[0].AsObject(ctx).GetClass()
		} else if args[0].GetType() == phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"ReflectionMethod::__construct(): Argument #1 ($objectOrMethod) must be of type object|string, array given")
		} else {
			className := args[0].AsString(ctx)
			class, err = resolveClass(ctx, className)
			if err != nil {
				return nil, err
			}
		}
		// Check methodName type
		if len(args) > 1 && args[1].GetType() == phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"ReflectionMethod::__construct(): Argument #2 ($method) must be of type ?string, array given")
		}
		methodName = args[1].AsString(ctx)
	}

	method, ok := class.GetMethod(methodName)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s() does not exist", class.GetName(), methodName))
	}

	data := &reflectionMethodData{
		method: method,
		class:  class,
	}

	// When reflecting __invoke on a Closure instance, capture the underlying
	// callable so that getParameters() can return the closure's actual parameters
	// (not the parameters of the NativeMethod wrapper).
	if args[0].GetType() == phpv.ZtObject && methodName == "__invoke" {
		obj, ok2 := args[0].AsObject(ctx).(*phpobj.ZObject)
		if ok2 {
			// Iterate through opaque map to find FuncGetArgs implementor
			for _, v := range obj.Opaque {
				if fga, ok3 := v.(phpv.FuncGetArgs); ok3 {
					data.closureCallable = fga
					break
				}
			}
		}
	}

	// The "class" property should show the declaring class
	declaringClassName := class.GetName()
	if method.Class != nil {
		declaringClassName = method.Class.GetName()
	}

	o.HashTable().SetString("name", method.Name.ZVal())
	o.HashTable().SetString("class", declaringClassName.ZVal())
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
	// For closures reflected via __invoke, use the closure's actual args.
	if data.closureCallable != nil {
		return phpv.ZInt(len(data.closureCallable.GetArgs())).ZVal(), nil
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
	// For closures reflected via __invoke, use the closure's actual args.
	var argList []*phpv.FuncArg
	if data.closureCallable != nil {
		argList = data.closureCallable.GetArgs()
	} else if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		argList = fga.GetArgs()
	}
	count := 0
	for _, a := range argList {
		if a.Required {
			count++
		}
	}
	return phpv.ZInt(count).ZVal(), nil
}

func reflectionMethodGetParameters(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	// When reflecting __invoke on a Closure, use the closure's actual args.
	if data.closureCallable != nil {
		funcName := phpv.ZString(string(data.class.GetName()) + "::" + string(data.method.Name))
		return createReflectionParameterObjectsWithClass(ctx, data.closureCallable.GetArgs(), funcName, nil, data.class)
	}
	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		funcName := phpv.ZString(string(data.class.GetName()) + "::" + string(data.method.Name))
		return createReflectionParameterObjectsWithClass(ctx, fga.GetArgs(), funcName, nil, data.class)
	}
	return phpv.NewZArray().ZVal(), nil
}

func reflectionMethodInvoke(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Cannot invoke abstract methods
	if data.method.Modifiers.Has(phpv.ZAttrAbstract) {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke abstract method %s::%s()", data.class.GetName(), data.method.Name))
	}

	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionMethod::invoke() expects at least 1 argument, 0 given")
	}

	// First argument is the object instance (or null for static methods)
	objArg := args[0]
	methodArgs := args[1:]

	// Validate first argument type - must be ?object
	if objArg.GetType() != phpv.ZtObject && objArg.GetType() != phpv.ZtNull {
		typeName := objArg.GetType().String()
		switch objArg.GetType() {
		case phpv.ZtBool:
			if objArg.AsBool(ctx) {
				typeName = "true"
			} else {
				typeName = "false"
			}
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionMethod::invoke(): Argument #1 ($object) must be of type ?object, %s given", typeName))
	}

	if data.method.Modifiers.IsStatic() {
		// For static methods, always call without $this regardless of the object argument.
		// PHP ignores any passed object for static methods.
		// Use BindClassLSB to set the called class for late static binding (static::).
		// data.class is the class on which the method was reflected (e.g. B when reflecting B::call),
		// data.method.Class is the declaring class (e.g. A).
		callable := phpv.BindClassLSB(data.method.Method, data.class, data.class, true)
		return ctx.CallZValInternal(ctx, callable, methodArgs)
	}

	// Check for non-object argument (null for non-static method)
	if objArg.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke non static method %s::%s() without an object", data.class.GetName(), data.method.Name))
	}

	obj := objArg.AsObject(ctx)
	// Check that the object is an instance of the declaring class
	declaringClass := data.class
	if data.method.Class != nil {
		declaringClass = data.method.Class
	}
	if !obj.GetClass().InstanceOf(declaringClass) {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Given object is not an instance of the class this method was declared in")
	}
	// Use CallZValInternal so the call appears as [internal function] in stack traces
	return ctx.CallZValInternal(ctx, data.method.Method, methodArgs, obj)
}

func reflectionMethodInvokeArgs(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Cannot invoke abstract methods
	if data.method.Modifiers.Has(phpv.ZAttrAbstract) {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke abstract method %s::%s()", data.class.GetName(), data.method.Name))
	}

	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::invokeArgs() expects exactly 2 arguments")
	}

	// First argument is the object instance (or null for static methods)
	objArg := args[0]

	// Validate first argument type - must be ?object
	if objArg.GetType() != phpv.ZtObject && objArg.GetType() != phpv.ZtNull {
		typeName := objArg.GetType().String()
		switch objArg.GetType() {
		case phpv.ZtBool:
			if objArg.AsBool(ctx) {
				typeName = "true"
			} else {
				typeName = "false"
			}
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionMethod::invokeArgs(): Argument #1 ($object) must be of type ?object, %s given", typeName))
	}

	// Second argument must be an array
	if args[1].GetType() != phpv.ZtArray {
		typeName := args[1].GetType().String()
		switch args[1].GetType() {
		case phpv.ZtBool:
			if args[1].AsBool(ctx) {
				typeName = "true"
			} else {
				typeName = "false"
			}
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionMethod::invokeArgs(): Argument #2 ($args) must be of type array, %s given", typeName))
	}
	arr := args[1].Value().(*phpv.ZArray)
	var callArgs []*phpv.ZVal
	for _, v := range arr.Iterate(ctx) {
		callArgs = append(callArgs, v)
	}

	if data.method.Modifiers.IsStatic() {
		// Use BindClassLSB to support late static binding (static::).
		callable := phpv.BindClassLSB(data.method.Method, data.class, data.class, true)
		if objArg.GetType() == phpv.ZtObject {
			return ctx.CallZValInternal(ctx, callable, callArgs, objArg.AsObject(ctx))
		}
		return ctx.CallZValInternal(ctx, callable, callArgs)
	}

	if objArg.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke non static method %s::%s() without an object", data.class.GetName(), data.method.Name))
	}

	obj := objArg.AsObject(ctx)
	// Check that the object is an instance of the declaring class
	declaringClass := data.class
	if data.method.Class != nil {
		declaringClass = data.method.Class
	}
	if !obj.GetClass().InstanceOf(declaringClass) {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Given object is not an instance of the class this method was declared in")
	}
	return ctx.CallZValInternal(ctx, data.method.Method, callArgs, obj)
}

// createReflectionMethodObject creates a ReflectionMethod object for the given
// class and method, without going through __construct.
func createReflectionMethodObject(ctx phpv.Context, class phpv.ZClass, method *phpv.ZClassMethod) (*phpv.ZVal, error) {
	return createReflectionMethodObjectWithInstance(ctx, class, method, nil)
}

func createReflectionMethodObjectWithInstance(ctx phpv.Context, class phpv.ZClass, method *phpv.ZClassMethod, instance phpv.ZObject) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionMethod)
	if err != nil {
		return nil, err
	}
	data := &reflectionMethodData{
		method: method,
		class:  class,
	}

	// When reflecting __invoke on a Closure instance, capture the closure's args
	// so that getParameters()/getNumberOfParameters() work correctly.
	if instance != nil && method.Name == "__invoke" {
		if zo, ok := instance.(*phpobj.ZObject); ok {
			for _, v := range zo.Opaque {
				if fga, ok2 := v.(phpv.FuncGetArgs); ok2 {
					data.closureCallable = fga
					break
				}
			}
		}
	}

	// The "class" property should show the declaring class (where the method was actually defined)
	declaringClassName := class.GetName()
	if method.Class != nil {
		declaringClassName = method.Class.GetName()
	}

	obj.HashTable().SetString("name", method.Name.ZVal())
	obj.HashTable().SetString("class", declaringClassName.ZVal())
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
		// Validate that the instance is of the correct class
		if instance != nil && !instance.GetClass().InstanceOf(data.class) {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				"Given object is not an instance of the class this method was declared in")
		}
	}

	// Build an array callable [$instance, "methodName"] or ["ClassName", "methodName"]
	// and use Closure::fromCallable to create a proper Closure object.
	// Use the declaring class (method.Class) if available, as it's the real owner of the method.
	arr := phpv.NewZArray()
	if instance != nil {
		arr.OffsetSet(ctx, phpv.ZInt(0), instance.ZVal())
	} else {
		className := data.class.GetName()
		if data.method.Class != nil {
			className = data.method.Class.GetName()
		}
		arr.OffsetSet(ctx, phpv.ZInt(0), className.ZVal())
	}
	arr.OffsetSet(ctx, phpv.ZInt(1), data.method.Name.ZVal())
	return closureFromCallableVal(ctx, arr.ZVal())
}

func reflectionMethodGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.method.Attributes, phpobj.AttributeTARGET_METHOD, name, flags, data.class)
}

func reflectionMethodGetPrototype(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Walk up parent classes and interfaces to find a prototype
	methodNameLower := data.method.Name.ToLower()

	// Check parent class chain
	zc, ok := data.class.(*phpobj.ZClass)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s does not have a prototype", data.class.GetName(), data.method.Name))
	}

	// Check parent classes
	if zc.Extends != nil {
		if m, ok := zc.Extends.GetMethod(methodNameLower); ok {
			return createReflectionMethodObject(ctx, zc.Extends, m)
		}
	}

	// Check interfaces
	for _, impl := range zc.Implementations {
		if m, ok := impl.GetMethod(methodNameLower); ok {
			return createReflectionMethodObject(ctx, impl, m)
		}
	}

	return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s does not have a prototype", data.class.GetName(), data.method.Name))
}

func reflectionMethodIsDestructor(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Name.ToLower() == "__destruct").ZVal(), nil
}

func reflectionMethodIsInternal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Loc == nil).ZVal(), nil
}

func reflectionMethodIsUserDefined(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.Loc != nil).ZVal(), nil
}

func reflectionMethodGetModifiers(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	var mods int64
	access := data.method.Modifiers.Access()
	switch access {
	case phpv.ZAttrProtected:
		mods |= ReflectionMethodIS_PROTECTED
	case phpv.ZAttrPrivate:
		mods |= ReflectionMethodIS_PRIVATE
	default:
		mods |= ReflectionMethodIS_PUBLIC
	}
	if data.method.Modifiers.IsStatic() {
		mods |= ReflectionMethodIS_STATIC
	}
	if data.method.Modifiers.Has(phpv.ZAttrFinal) {
		mods |= ReflectionMethodIS_FINAL
	}
	if data.method.Modifiers.Has(phpv.ZAttrAbstract) || data.method.Empty {
		mods |= ReflectionMethodIS_ABSTRACT
	}
	return phpv.ZInt(mods).ZVal(), nil
}

func reflectionMethodGetFileName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil || data.method.Loc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(data.method.Loc.Filename).ZVal(), nil
}

func reflectionMethodGetStartLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil || data.method.Loc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(data.method.Loc.Line).ZVal(), nil
}

func reflectionMethodGetEndLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil || data.method.Loc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.method.LocEnd != nil {
		return phpv.ZInt(data.method.LocEnd.Line).ZVal(), nil
	}
	// fallback: try to get end line from the callable
	type locEndGetter interface {
		LocEnd() *phpv.Loc
	}
	if leg, ok := data.method.Method.(locEndGetter); ok {
		if loc := leg.LocEnd(); loc != nil {
			return phpv.ZInt(loc.Line).ZVal(), nil
		}
	}
	return phpv.ZInt(data.method.Loc.Line).ZVal(), nil
}

func reflectionMethodReturnsReference(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type refGetter interface {
		ReturnsByRef() bool
	}
	if rg, ok := data.method.Method.(refGetter); ok {
		return phpv.ZBool(rg.ReturnsByRef()).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodIsVariadic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		for _, arg := range fga.GetArgs() {
			if arg.Variadic {
				return phpv.ZBool(true).ZVal(), nil
			}
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodGetStaticVariables(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	arr := phpv.NewZArray()
	if data == nil || data.method == nil || data.method.Method == nil {
		return arr.ZVal(), nil
	}
	if svg, ok := data.method.Method.(phpv.StaticVarGetter); ok {
		for _, entry := range svg.GetStaticVars(ctx) {
			arr.OffsetSet(ctx, entry.Name.ZVal(), entry.Val)
		}
	}
	return arr.ZVal(), nil
}

func reflectionMethodGetExtensionName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodSetAccessible(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// setAccessible has no effect since PHP 8.1, deprecated since 8.5
	_ = ctx.Deprecated("Method ReflectionMethod::setAccessible() is deprecated since 8.5, as it has no effect since PHP 8.1", logopt.NoFuncName(true))
	return phpv.ZNULL.ZVal(), nil
}

func reflectionMethodToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZString("Method [ ]").ZVal(), nil
	}

	m := data.method
	class := data.class
	methodNameLower := m.Name.ToLower()
	isOwnMethod := m.Class == nil || m.Class.GetName() == class.GetName()
	isInternal := m.Loc == nil

	var sb strings.Builder
	sb.WriteString("Method [ ")

	origin := "<user"
	if isInternal {
		if zc, ok := class.(*phpobj.ZClass); ok && zc.Ext != "" {
			origin = "<internal:" + zc.Ext
		} else {
			origin = "<internal>"
			// For internal methods, skip the rest of origin building
			sb.WriteString(origin)
			goto writeModifiers
		}
	}

	if isOwnMethod {
		// Private methods do NOT show "overwrites" or "prototype" - they are not polymorphic
		if !m.Modifiers.IsPrivate() {
			// Check if this method overwrites a parent method
			if zc, ok := class.(*phpobj.ZClass); ok && zc.Extends != nil {
				if parentMethod, ok2 := zc.Extends.GetMethod(methodNameLower); ok2 {
					// Only show "overwrites" if the parent method is NOT private
					if !parentMethod.Modifiers.IsPrivate() {
						declaringClass := zc.Extends.GetName()
						if parentMethod.Class != nil {
							declaringClass = parentMethod.Class.GetName()
						}
						origin += ", overwrites " + string(declaringClass)
					}
				}
			}
			// Find prototype
			if zc, ok := class.(*phpobj.ZClass); ok {
				var protoName phpv.ZString
				if m.Prototype != nil {
					protoName = m.Prototype.GetName()
				} else {
					protoName = findMethodPrototype(zc, methodNameLower)
				}
				if protoName != "" {
					origin += ", prototype " + string(protoName)
				}
			}
		}
	} else {
		// Inherited method
		declaringClass := m.Class.GetName()
		origin += ", inherits " + string(declaringClass)
		// Find prototype - only show if different from declaring class
		// Private methods have no prototype
		if !m.Modifiers.IsPrivate() {
			var protoName phpv.ZString
			if m.Prototype != nil {
				protoName = m.Prototype.GetName()
			} else if zc, ok := class.(*phpobj.ZClass); ok {
				protoName = findMethodPrototype(zc, methodNameLower)
			}
			if protoName != "" && protoName != declaringClass {
				origin += ", prototype " + string(protoName)
			}
		}
	}

	// ctor suffix (PHP shows ctor for constructors but not dtor for destructors)
	{
		nameLower := strings.ToLower(string(m.Name))
		if nameLower == "__construct" {
			origin += ", ctor"
		}
	}
	origin += ">"
	sb.WriteString(origin)

writeModifiers:
	if m.Modifiers.Has(phpv.ZAttrAbstract) || m.Empty {
		sb.WriteString(" abstract")
	}
	if m.Modifiers.Has(phpv.ZAttrFinal) {
		sb.WriteString(" final")
	}

	if m.Modifiers.IsStatic() {
		sb.WriteString(" static")
	}

	access := m.Modifiers.Access()
	if access == phpv.ZAttrProtected {
		sb.WriteString(" protected")
	} else if access == phpv.ZAttrPrivate {
		sb.WriteString(" private")
	} else {
		sb.WriteString(" public")
	}

	sb.WriteString(fmt.Sprintf(" method %s ] {\n", m.Name))

	if m.Loc != nil {
		endLine := m.Loc.Line
		if m.LocEnd != nil {
			endLine = m.LocEnd.Line
		} else {
			type locEndGetter interface {
				LocEnd() *phpv.Loc
			}
			if leg, ok := m.Method.(locEndGetter); ok {
				if loc := leg.LocEnd(); loc != nil {
					endLine = loc.Line
				}
			}
		}
		sb.WriteString(fmt.Sprintf("  @@ %s %d - %d\n", m.Loc.Filename, m.Loc.Line, endLine))
	}

	if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
		funcArgs := fga.GetArgs()
		sb.WriteString(fmt.Sprintf("\n  - Parameters [%d] {\n", len(funcArgs)))
		for i, arg := range funcArgs {
			sb.WriteString(rcFormatParameter(ctx, i, arg, "    "))
		}
		sb.WriteString("  }\n")
	} else {
		// Methods without FuncGetArgs (internal or hook methods)
		sb.WriteString("\n  - Parameters [0] {\n")
		sb.WriteString("  }\n")
	}

	// Return type
	if m.ReturnType != nil {
		if m.TentativeReturnType {
			sb.WriteString(fmt.Sprintf("  - Tentative return [ %s ]\n", m.ReturnType.String()))
		} else {
			sb.WriteString(fmt.Sprintf("  - Return [ %s ]\n", m.ReturnType.String()))
		}
	}

	sb.WriteString("}\n")

	return phpv.ZString(sb.String()).ZVal(), nil
}

func reflectionMethodIsGenerator(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type generatorChecker interface {
		IsGenerator() bool
	}
	if gc, ok := data.method.Method.(generatorChecker); ok {
		return phpv.ZBool(gc.IsGenerator()).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodGetShortName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.method.Name.ZVal(), nil
}

func reflectionMethodGetNamespaceName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Methods don't have namespaces (only their classes do)
	return phpv.ZString("").ZVal(), nil
}

func reflectionMethodInNamespace(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

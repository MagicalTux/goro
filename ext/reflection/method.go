package reflection

import (
	"fmt"
	"strings"

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
		"invokeargs":                    {Name: "invokeArgs", Method: phpobj.NativeMethod(reflectionMethodInvokeArgs)},
		"getattributes":                 {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionMethodGetAttributes)},
		"getclosure":                    {Name: "getClosure", Method: phpobj.NativeMethod(reflectionMethodGetClosure)},
		"getdoccomment":                 {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionMethodGetDocComment)},
		"isdeprecated":                  {Name: "isDeprecated", Method: phpobj.NativeMethod(reflectionMethodIsDeprecated)},
		"getreturntype":                 {Name: "getReturnType", Method: phpobj.NativeMethod(reflectionMethodGetReturnType)},
		"hasreturntype":                 {Name: "hasReturnType", Method: phpobj.NativeMethod(reflectionMethodHasReturnType)},
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
	}
}

// reflectionMethodGetDocComment returns the doc comment for a method.
// Doc comments are not preserved during compilation, so this always returns false.
func reflectionMethodGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFalse.ZVal(), nil
}

func reflectionMethodConstructFull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::__construct() expects at least 1 argument, 0 given")
	}

	var class phpv.ZClass
	var methodName phpv.ZString
	var err error

	if len(args) == 1 {
		// Single argument form: "ClassName::methodName"
		methodStr := string(args[0].AsString(ctx))
		parts := strings.SplitN(methodStr, "::", 2)
		if len(parts) != 2 {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				fmt.Sprintf("ReflectionMethod::__construct(): Argument #1 ($objectOrMethod) must be a valid method name"))
		}

		// Emit deprecation notice (ignore error - it's just a notice)
		_ = ctx.Deprecated("Calling ReflectionMethod::__construct() with 1 argument is deprecated, use ReflectionMethod::createFromMethodName() instead")

		className := phpv.ZString(parts[0])
		methodName = phpv.ZString(parts[1])

		if string(className) == "" {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				"ReflectionMethod::__construct(): Argument #1 ($objectOrMethod) must be a valid method name")
		}

		class, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	} else {
		// Two argument form: (class/object, methodName)
		if args[0].GetType() == phpv.ZtObject {
			class = args[0].AsObject(ctx).GetClass()
		} else {
			className := args[0].AsString(ctx)
			class, err = resolveClass(ctx, className)
			if err != nil {
				return nil, err
			}
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
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionMethod::invoke() expects at least 1 argument, 0 given")
	}

	// First argument is the object instance (or null for static methods)
	objArg := args[0]
	methodArgs := args[1:]

	if data.method.Modifiers.IsStatic() {
		// For static methods, call without $this (but pass the object if provided)
		if objArg.GetType() == phpv.ZtObject {
			obj := objArg.AsObject(ctx)
			return ctx.CallZVal(ctx, data.method.Method, methodArgs, obj)
		}
		return ctx.CallZVal(ctx, data.method.Method, methodArgs)
	}

	// Check for non-object argument
	if objArg.GetType() != phpv.ZtObject {
		if objArg.GetType() == phpv.ZtNull {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Trying to invoke non static method %s::%s() without an object", data.class.GetName(), data.method.Name))
		}
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
	return ctx.CallZVal(ctx, data.method.Method, methodArgs, obj)
}

func reflectionMethodInvokeArgs(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::invokeArgs() expects exactly 2 arguments")
	}

	// First argument is the object instance (or null for static methods)
	objArg := args[0]

	// Second argument is the array of arguments
	arrVal, err := args[1].As(ctx, phpv.ZtArray)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionMethod::invokeArgs(): Argument #2 ($args) must be of type array")
	}
	arr := arrVal.Value().(*phpv.ZArray)
	var callArgs []*phpv.ZVal
	for _, v := range arr.Iterate(ctx) {
		callArgs = append(callArgs, v)
	}

	if data.method.Modifiers.IsStatic() {
		return ctx.CallZVal(ctx, data.method.Method, callArgs)
	}

	if objArg.GetType() != phpv.ZtObject || objArg.GetType() == phpv.ZtNull {
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
	return ctx.CallZVal(ctx, data.method.Method, callArgs, obj)
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
	return filterAttributes(ctx, data.method.Attributes, phpobj.AttributeTARGET_METHOD, name, flags)
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
	return phpv.ZInt(data.method.Loc.Line).ZVal(), nil
}

func reflectionMethodReturnsReference(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type refGetter interface {
		ReturnsRef() bool
	}
	if rg, ok := data.method.Method.(refGetter); ok {
		return phpv.ZBool(rg.ReturnsRef()).ZVal(), nil
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
	return phpv.NewZArray().ZVal(), nil
}

func reflectionMethodGetExtensionName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodSetAccessible(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// setAccessible has no effect since PHP 8.1, deprecated since 8.5
	_ = ctx.Deprecated("Method ReflectionMethod::setAccessible() is deprecated since 8.5, as it has no effect since PHP 8.1")
	return phpv.ZNULL.ZVal(), nil
}

func reflectionMethodToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZString("Method [ ]").ZVal(), nil
	}

	var sb strings.Builder
	sb.WriteString("Method [ ")

	origin := "<user>"
	if data.method.Loc == nil {
		origin = "<internal>"
	}
	if data.method.Class != nil && data.method.Class.GetName() != data.class.GetName() {
		origin += ", inherits " + string(data.method.Class.GetName())
	}
	sb.WriteString(origin)

	if data.method.Modifiers.Has(phpv.ZAttrAbstract) || data.method.Empty {
		sb.WriteString(" abstract")
	}
	if data.method.Modifiers.Has(phpv.ZAttrFinal) {
		sb.WriteString(" final")
	}

	access := data.method.Modifiers.Access()
	if access == phpv.ZAttrProtected {
		sb.WriteString(" protected")
	} else if access == phpv.ZAttrPrivate {
		sb.WriteString(" private")
	} else {
		sb.WriteString(" public")
	}

	if data.method.Modifiers.IsStatic() {
		sb.WriteString(" static")
	}

	sb.WriteString(fmt.Sprintf(" method %s ] {\n", data.method.Name))

	if data.method.Loc != nil {
		sb.WriteString(fmt.Sprintf("  @@ %s %d - %d\n", data.method.Loc.Filename, data.method.Loc.Line, data.method.Loc.Line))
	}

	if fga, ok := data.method.Method.(phpv.FuncGetArgs); ok {
		funcArgs := fga.GetArgs()
		required := 0
		for _, a := range funcArgs {
			if a.Required {
				required++
			}
		}
		sb.WriteString(fmt.Sprintf("\n  - Parameters [%d] {\n", len(funcArgs)))
		for i, arg := range funcArgs {
			sb.WriteString(fmt.Sprintf("    Parameter #%d [ ", i))
			if !arg.Required {
				sb.WriteString("<optional> ")
			} else {
				sb.WriteString("<required> ")
			}
			if arg.Hint != nil {
				sb.WriteString(arg.Hint.String() + " ")
			}
			sb.WriteString(fmt.Sprintf("$%s", arg.VarName))
			sb.WriteString(" ]\n")
		}
		sb.WriteString("  }\n")
	}
	sb.WriteString("}\n")

	return phpv.ZString(sb.String()).ZVal(), nil
}

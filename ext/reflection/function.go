package reflection

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionFunction class

var ReflectionFunction *phpobj.ZClass

// reflectionFunctionData is stored as opaque data on ReflectionFunction objects
type reflectionFunctionData struct {
	name       phpv.ZString
	callable   phpv.Callable
	args       []*phpv.FuncArg // may be nil if callable doesn't implement FuncGetArgs
	closureObj *phpv.ZVal      // the original Closure ZVal (nil for named functions)
	closure    phpv.ZClosure   // the ZClosure interface (nil for named functions)
}

func initReflectionFunction() {
	ReflectionFunction = &phpobj.ZClass{
		Name: "ReflectionFunction",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":                   {Name: "__construct", Method: phpobj.NativeMethod(reflectionFunctionConstruct)},
			"getname":                       {Name: "getName", Method: phpobj.NativeMethod(reflectionFunctionGetName)},
			"getshortname":                  {Name: "getShortName", Method: phpobj.NativeMethod(reflectionFunctionGetShortName)},
			"getnamespacename":              {Name: "getNamespaceName", Method: phpobj.NativeMethod(reflectionFunctionGetNamespaceName)},
			"innamespace":                   {Name: "inNamespace", Method: phpobj.NativeMethod(reflectionFunctionInNamespace)},
			"getnumberofparameters":         {Name: "getNumberOfParameters", Method: phpobj.NativeMethod(reflectionFunctionGetNumberOfParameters)},
			"getnumberofrequiredparameters": {Name: "getNumberOfRequiredParameters", Method: phpobj.NativeMethod(reflectionFunctionGetNumberOfRequiredParameters)},
			"getparameters":                 {Name: "getParameters", Method: phpobj.NativeMethod(reflectionFunctionGetParameters)},
			"invoke":                        {Name: "invoke", Method: phpobj.NativeMethod(reflectionFunctionInvoke)},
			"invokeargs":                    {Name: "invokeArgs", Method: phpobj.NativeMethod(reflectionFunctionInvokeArgs)},
			"getattributes":                 {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionFunctionGetAttributes)},
			"getclosure":                    {Name: "getClosure", Method: phpobj.NativeMethod(reflectionFunctionGetClosure)},
			"getclosurethis":                {Name: "getClosureThis", Method: phpobj.NativeMethod(reflectionFunctionGetClosureThis)},
			"getclosurescopeclass":          {Name: "getClosureScopeClass", Method: phpobj.NativeMethod(reflectionFunctionGetClosureScopeClass)},
			"getclosureusedvariables":       {Name: "getClosureUsedVariables", Method: phpobj.NativeMethod(reflectionFunctionGetClosureUsedVariables)},
			"isstatic":                      {Name: "isStatic", Method: phpobj.NativeMethod(reflectionFunctionIsStatic)},
			"isclosure":                     {Name: "isClosure", Method: phpobj.NativeMethod(reflectionFunctionIsClosure)},
			"getreturntype":                 {Name: "getReturnType", Method: phpobj.NativeMethod(reflectionFunctionGetReturnType)},
			"getdoccomment":                 {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionFunctionGetDocComment)},
			"isdeprecated":                  {Name: "isDeprecated", Method: phpobj.NativeMethod(reflectionFunctionIsDeprecated)},
			"isvariadic":                    {Name: "isVariadic", Method: phpobj.NativeMethod(reflectionFunctionIsVariadic)},
			"isanonymous":                   {Name: "isAnonymous", Method: phpobj.NativeMethod(reflectionFunctionIsAnonymous)},
			"hasreturntype":                 {Name: "hasReturnType", Method: phpobj.NativeMethod(reflectionFunctionHasReturnType)},
			"getfilename":                   {Name: "getFileName", Method: phpobj.NativeMethod(reflectionFunctionGetFileName)},
			"getextensionname":              {Name: "getExtensionName", Method: phpobj.NativeMethod(reflectionFunctionGetExtensionName)},
			"getstaticvariables":            {Name: "getStaticVariables", Method: phpobj.NativeMethod(reflectionFunctionGetStaticVariables)},
			"isgenerator":                   {Name: "isGenerator", Method: phpobj.NativeMethod(reflectionFunctionIsGenerator)},
			"isdisabled":                    {Name: "isDisabled", Method: phpobj.NativeMethod(reflectionFunctionIsDisabled)},
			"getextension":                  {Name: "getExtension", Method: phpobj.NativeMethod(reflectionFunctionGetExtension)},
			"getclosurecalledclass":         {Name: "getClosureCalledClass", Method: phpobj.NativeMethod(reflectionFunctionGetClosureCalledClass)},
			"returnsreference":              {Name: "returnsReference", Method: phpobj.NativeMethod(reflectionFunctionReturnsReference)},
			"__tostring":                    {Name: "__toString", Method: phpobj.NativeMethod(reflectionFunctionToString)},
			"getstartline":                  {Name: "getStartLine", Method: phpobj.NativeMethod(reflectionFunctionGetStartLine)},
			"getendline":                    {Name: "getEndLine", Method: phpobj.NativeMethod(reflectionFunctionGetEndLine)},
			"isinternal":                    {Name: "isInternal", Method: phpobj.NativeMethod(reflectionFunctionIsInternal)},
			"isuserdefined":                 {Name: "isUserDefined", Method: phpobj.NativeMethod(reflectionFunctionIsUserDefined)},
		},
	}
}

// reflectionFunctionGetDocComment returns the doc comment for a function.
// Doc comments are not preserved during compilation, so this always returns false.
func reflectionFunctionGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFalse.ZVal(), nil
}

func reflectionFunctionConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionFunction::__construct() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionFunction::__construct() expects exactly 1 argument, %d given", len(args)))
	}

	arg := args[0]

	// Check for invalid argument types
	if arg.GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionFunction::__construct(): Argument #1 ($function) must be of type Closure|string, array given")
	}

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
				data.closureObj = arg
				data.closure = closure
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

func reflectionFunctionInvokeArgs(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.callable == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionFunction::invokeArgs() expects exactly 1 argument, 0 given")
	}
	// args[0] should be an array of arguments
	arrVal, err := args[0].As(ctx, phpv.ZtArray)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionFunction::invokeArgs(): Argument #1 ($args) must be of type array")
	}
	arr := arrVal.Value().(*phpv.ZArray)
	var callArgs []*phpv.ZVal
	for _, v := range arr.Iterate(ctx) {
		callArgs = append(callArgs, v)
	}
	return ctx.CallZVal(ctx, data.callable, callArgs)
}

func reflectionFunctionGetShortName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	name := string(data.name)
	// Closures (names starting with "{closure:") should not be split on namespace separator
	if strings.HasPrefix(name, "{closure:") {
		return phpv.ZString(name).ZVal(), nil
	}
	// Find last \ for namespace separator
	if idx := lastIndexByte(name, '\\'); idx >= 0 {
		return phpv.ZString(name[idx+1:]).ZVal(), nil
	}
	return phpv.ZString(name).ZVal(), nil
}

func reflectionFunctionGetNamespaceName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	name := string(data.name)
	// Closures don't have a namespace component
	if strings.HasPrefix(name, "{closure:") {
		return phpv.ZString("").ZVal(), nil
	}
	if idx := lastIndexByte(name, '\\'); idx >= 0 {
		return phpv.ZString(name[:idx]).ZVal(), nil
	}
	return phpv.ZString("").ZVal(), nil
}

func reflectionFunctionInNamespace(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := string(data.name)
	// Closures are not "in a namespace" even if their name contains backslashes
	if strings.HasPrefix(name, "{closure:") {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(lastIndexByte(name, '\\') >= 0).ZVal(), nil
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func reflectionFunctionGetClosure(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if data.closureObj != nil {
		return data.closureObj, nil
	}
	// For named functions, wrap in a Closure
	if data.callable != nil {
		return closureFromCallableHelper(ctx, data.callable, data.name, data.args)
	}
	return phpv.ZNULL.ZVal(), nil
}

func reflectionFunctionGetClosureThis(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.closure == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	this := data.closure.GetThis()
	if this == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return this.ZVal(), nil
}

func reflectionFunctionGetClosureScopeClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.closure == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	class := data.closure.GetClass()
	if class == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// Return a ReflectionClass for this scope class
	rcObj, err := phpobj.CreateZObject(ctx, ReflectionClass)
	if err != nil {
		return nil, err
	}
	rcObj.HashTable().SetString("name", class.GetName().ZVal())
	rcObj.SetOpaque(ReflectionClass, class)
	return rcObj.ZVal(), nil
}

func reflectionFunctionGetClosureUsedVariables(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.closure == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	// Try to get used variables from the closure
	type useGetter interface {
		GetUseVars() []*phpv.FuncUse
	}
	if ug, ok := data.closure.(useGetter); ok {
		vars := ug.GetUseVars()
		arr := phpv.NewZArray()
		for _, u := range vars {
			val := u.Value
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
			arr.OffsetSet(ctx, u.VarName, val)
		}
		return arr.ZVal(), nil
	}
	return phpv.NewZArray().ZVal(), nil
}

func reflectionFunctionIsStatic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.closure == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.closure.IsStatic()).ZVal(), nil
}

func reflectionFunctionIsClosure(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.closure != nil).ZVal(), nil
}

func reflectionFunctionGetReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	type returnTypeGetter interface {
		GetReturnType() *phpv.TypeHint
	}
	if rtg, ok := data.callable.(returnTypeGetter); ok {
		rt := rtg.GetReturnType()
		if rt != nil {
			return createReflectionTypeObject(ctx, rt)
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// closureFromCallableHelper wraps a Callable into a Closure object for ReflectionFunction::getClosure()
func closureFromCallableHelper(ctx phpv.Context, callable phpv.Callable, name phpv.ZString, funcArgs []*phpv.FuncArg) (*phpv.ZVal, error) {
	// Use Closure::fromCallable to create a proper Closure object
	return closureFromCallableVal(ctx, name.ZVal())
}

func reflectionFunctionIsInternal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// A function is internal if it doesn't have a location (i.e., not user-defined)
	type locGetter interface {
		Loc() *phpv.Loc
	}
	if lg, ok := data.callable.(locGetter); ok {
		loc := lg.Loc()
		if loc != nil && loc.Filename != "" {
			return phpv.ZBool(false).ZVal(), nil
		}
	}
	// If no closure and no Loc, it's internal
	if data.closure != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionFunctionIsUserDefined(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type locGetter interface {
		Loc() *phpv.Loc
	}
	if lg, ok := data.callable.(locGetter); ok {
		loc := lg.Loc()
		if loc != nil && loc.Filename != "" {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	if data.closure != nil {
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

// closureFromCallableVal is a helper that calls through to the Closure class's fromCallable method
func closureFromCallableVal(ctx phpv.Context, val *phpv.ZVal) (*phpv.ZVal, error) {
	// Resolve the Closure class
	cls, err := ctx.Global().GetClass(ctx, "Closure", false)
	if err != nil {
		return nil, err
	}
	method, ok := cls.GetMethod("fromcallable")
	if !ok {
		return nil, fmt.Errorf("Closure::fromCallable not found")
	}
	return ctx.CallZVal(ctx, method.Method, []*phpv.ZVal{val})
}

func reflectionFunctionGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Try to get attributes from the callable
	type attrGetter interface {
		GetAttributes() []*phpv.ZAttribute
	}
	var attrs []*phpv.ZAttribute
	if ag, ok := data.callable.(attrGetter); ok {
		attrs = ag.GetAttributes()
	}

	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, attrs, phpobj.AttributeTARGET_FUNCTION, name, flags)
}

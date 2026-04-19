package reflection

import (
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// arg helpers to create FuncArg more concisely
func requiredArg(name string, hint string) *phpv.FuncArg {
	var h *phpv.TypeHint
	if hint != "" {
		h = phpv.ParseTypeHint(phpv.ZString(hint))
	}
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: true, Hint: h}
}

func optionalArgNull(name string, hint string) *phpv.FuncArg {
	var h *phpv.TypeHint
	if hint != "" {
		h = phpv.ParseTypeHint(phpv.ZString(hint))
	}
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: false, Hint: h, DefaultValue: phpv.ZNULL}
}

func optionalArgInt(name string, hint string, val int64) *phpv.FuncArg {
	var h *phpv.TypeHint
	if hint != "" {
		h = phpv.ParseTypeHint(phpv.ZString(hint))
	}
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: false, Hint: h, DefaultValue: phpv.ZInt(val)}
}

func optionalArgArray(name string) *phpv.FuncArg {
	h := phpv.ParseTypeHint(phpv.ZString("array"))
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: false, Hint: h, DefaultValue: phpv.NewZArray()}
}

func optionalArgDefault(name string, hint string) *phpv.FuncArg {
	var h *phpv.TypeHint
	if hint != "" {
		h = phpv.ParseTypeHint(phpv.ZString(hint))
	}
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: false, Hint: h}
}

func variadicArg(name string, hint string) *phpv.FuncArg {
	var h *phpv.TypeHint
	if hint != "" {
		h = phpv.ParseTypeHint(phpv.ZString(hint))
	}
	return &phpv.FuncArg{VarName: phpv.ZString(name), Required: false, Variadic: true, Hint: h}
}

func namedMethod(fn phpobj.NativeMethod, args ...*phpv.FuncArg) *phpobj.NativeMethodNamed {
	return &phpobj.NativeMethodNamed{Fn: fn, Args: args}
}

func initReflectionClass() {
	// ReflectionClass is declared in ext.go; we extend its methods here
	ReflectionClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__clone": {Name: "__clone", Modifiers: phpv.ZAttrPrivate,
			Method:     phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) { return nil, nil }),
			ReturnType: phpv.ParseTypeHint("void")},
		"__construct": {Name: "__construct", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassConstruct,
				// Type hint for display: object|string. Validation done in function body
				// to handle deprecated null (PHP emits deprecation, not TypeError for null).
				&phpv.FuncArg{VarName: "objectOrClass", Required: true, Hint: phpv.ParseTypeHint("object|string"), SkipTypeCheck: true},
			)},
		"__tostring": {Name: "__toString", Modifiers: phpv.ZAttrPublic,
			Method:     phpobj.NativeMethod(reflectionClassToString),
			ReturnType: phpv.ParseTypeHint("string")},
		"getname": {Name: "getName", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetName), ReturnType: phpv.ParseTypeHint("string"), TentativeReturnType: true},
		"isinternal": {Name: "isInternal", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsInternal), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isuserdefined": {Name: "isUserDefined", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsUserDefined), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isanonymous": {Name: "isAnonymous", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsAnonymous), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isinstantiable": {Name: "isInstantiable", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsInstantiable), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"iscloneable": {Name: "isCloneable", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsCloneable), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getfilename": {Name: "getFileName", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetFileName), ReturnType: phpv.ParseTypeHint("string|false"), TentativeReturnType: true},
		"getstartline": {Name: "getStartLine", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetStartLine), ReturnType: phpv.ParseTypeHint("int|false"), TentativeReturnType: true},
		"getendline": {Name: "getEndLine", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetEndLine), ReturnType: phpv.ParseTypeHint("int|false"), TentativeReturnType: true},
		"getdoccomment": {Name: "getDocComment", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetDocComment), ReturnType: phpv.ParseTypeHint("string|false"), TentativeReturnType: true},
		"getconstructor": {Name: "getConstructor", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetConstructor), ReturnType: phpv.ParseTypeHint("?ReflectionMethod"), TentativeReturnType: true},
		"hasmethod": {Name: "hasMethod", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassHasMethod,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getmethod": {Name: "getMethod", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetMethod,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("ReflectionMethod"), TentativeReturnType: true},
		"getmethods": {Name: "getMethods", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetMethods,
				optionalArgNull("filter", "?int"),
			), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"hasproperty": {Name: "hasProperty", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassHasProperty,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getproperty": {Name: "getProperty", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetProperty,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("ReflectionProperty"), TentativeReturnType: true},
		"getproperties": {Name: "getProperties", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetProperties,
				optionalArgNull("filter", "?int"),
			), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"hasconstant": {Name: "hasConstant", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassHasConstant,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getconstants": {Name: "getConstants", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetConstants,
				optionalArgNull("filter", "?int"),
			), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"getreflectionconstants": {Name: "getReflectionConstants", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetReflectionConstants,
				optionalArgNull("filter", "?int"),
			), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"getconstant": {Name: "getConstant", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetConstant,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("mixed"), TentativeReturnType: true},
		"getreflectionconstant": {Name: "getReflectionConstant", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetReflectionConstant,
				requiredArg("name", "string"),
			), ReturnType: phpv.ParseTypeHint("ReflectionClassConstant|false"), TentativeReturnType: true},
		"getinterfaces": {Name: "getInterfaces", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetInterfaces), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"getinterfacenames": {Name: "getInterfaceNames", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetInterfaceNames), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"isinterface": {Name: "isInterface", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsInterface), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"gettraits": {Name: "getTraits", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetTraits), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"gettraitnames": {Name: "getTraitNames", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetTraitNames), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"gettraitaliases": {Name: "getTraitAliases", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetTraitAliases), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"istrait": {Name: "isTrait", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsTrait), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isenum": {Name: "isEnum", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsEnum), ReturnType: phpv.ParseTypeHint("bool")},
		"isabstract": {Name: "isAbstract", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsAbstract), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isfinal": {Name: "isFinal", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsFinal), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isreadonly": {Name: "isReadOnly", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsReadOnly), ReturnType: phpv.ParseTypeHint("bool")},
		"getmodifiers": {Name: "getModifiers", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetModifiers), ReturnType: phpv.ParseTypeHint("int"), TentativeReturnType: true},
		"isinstance": {Name: "isInstance", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassIsInstance,
				requiredArg("object", "object"),
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"newinstance": {Name: "newInstance", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassNewInstance,
				variadicArg("args", "mixed"),
			), ReturnType: phpv.ParseTypeHint("object"), TentativeReturnType: true},
		"newinstancewithoutconstructor": {Name: "newInstanceWithoutConstructor", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassNewInstanceWithoutConstructor), ReturnType: phpv.ParseTypeHint("object"), TentativeReturnType: true},
		"newinstanceargs": {Name: "newInstanceArgs", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassNewInstanceArgs,
				optionalArgArray("args"),
			), ReturnType: phpv.ParseTypeHint("?object"), TentativeReturnType: true},
		// newLazyGhost, newLazyProxy, resetAsLazyGhost, resetAsLazyProxy,
		// initializeLazyObject, isUninitializedLazyObject, markLazyObjectAsInitialized,
		// getLazyInitializer are set by initLazyObjectMethods() in lazy.go
		"getparentclass": {Name: "getParentClass", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetParentClass), ReturnType: phpv.ParseTypeHint("ReflectionClass|false"), TentativeReturnType: true},
		"issubclassof": {Name: "isSubclassOf", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassIsSubclassOf,
				// SkipTypeCheck: validation done in function body to handle
				// deprecated null (PHP emits deprecation, not TypeError for null).
				&phpv.FuncArg{VarName: "class", Required: true, Hint: phpv.ParseTypeHint("ReflectionClass|string"), SkipTypeCheck: true},
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getstaticproperties": {Name: "getStaticProperties", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetStaticProperties), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"getstaticpropertyvalue": {Name: "getStaticPropertyValue", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetStaticPropertyValue,
				requiredArg("name", "string"),
				optionalArgDefault("default", "mixed"),
			), ReturnType: phpv.ParseTypeHint("mixed"), TentativeReturnType: true},
		"setstaticpropertyvalue": {Name: "setStaticPropertyValue", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassSetStaticPropertyValue,
				requiredArg("name", "string"),
				requiredArg("value", "mixed"),
			), ReturnType: phpv.ParseTypeHint("void"), TentativeReturnType: true},
		"getdefaultproperties": {Name: "getDefaultProperties", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetDefaultProperties), ReturnType: phpv.ParseTypeHint("array"), TentativeReturnType: true},
		"isiterable": {Name: "isIterable", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsIterable), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"isiterateable": {Name: "isIterateable", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassIsIterable), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"implementsinterface": {Name: "implementsInterface", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassImplementsInterface,
				requiredArg("interface", "ReflectionClass|string"),
			), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getextension": {Name: "getExtension", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetExtension), ReturnType: phpv.ParseTypeHint("?ReflectionExtension"), TentativeReturnType: true},
		"getextensionname": {Name: "getExtensionName", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetExtensionName), ReturnType: phpv.ParseTypeHint("string|false"), TentativeReturnType: true},
		"innamespace": {Name: "inNamespace", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassInNamespace), ReturnType: phpv.ParseTypeHint("bool"), TentativeReturnType: true},
		"getnamespacename": {Name: "getNamespaceName", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetNamespaceName), ReturnType: phpv.ParseTypeHint("string"), TentativeReturnType: true},
		"getshortname": {Name: "getShortName", Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(reflectionClassGetShortName), ReturnType: phpv.ParseTypeHint("string"), TentativeReturnType: true},
		"getattributes": {Name: "getAttributes", Modifiers: phpv.ZAttrPublic,
			Method: namedMethod(reflectionClassGetAttributes,
				optionalArgNull("name", "?string"),
				optionalArgInt("flags", "int", 0),
			), ReturnType: phpv.ParseTypeHint("array")},
	}

	// Set MethodOrder to match PHP's declaration order for ReflectionClass
	ReflectionClass.MethodOrder = []phpv.ZString{
		"__clone",
		"__construct",
		"__toString",
		"getName",
		"isInternal",
		"isUserDefined",
		"isAnonymous",
		"isInstantiable",
		"isCloneable",
		"getFileName",
		"getStartLine",
		"getEndLine",
		"getDocComment",
		"getConstructor",
		"hasMethod",
		"getMethod",
		"getMethods",
		"hasProperty",
		"getProperty",
		"getProperties",
		"hasConstant",
		"getConstants",
		"getReflectionConstants",
		"getConstant",
		"getReflectionConstant",
		"getInterfaces",
		"getInterfaceNames",
		"isInterface",
		"getTraits",
		"getTraitNames",
		"getTraitAliases",
		"isTrait",
		"isEnum",
		"isAbstract",
		"isFinal",
		"isReadOnly",
		"getModifiers",
		"isInstance",
		"newInstance",
		"newInstanceWithoutConstructor",
		"newInstanceArgs",
		"newLazyGhost",
		"newLazyProxy",
		"resetAsLazyGhost",
		"resetAsLazyProxy",
		"initializeLazyObject",
		"isUninitializedLazyObject",
		"markLazyObjectAsInitialized",
		"getLazyInitializer",
		"getParentClass",
		"isSubclassOf",
		"getStaticProperties",
		"getStaticPropertyValue",
		"setStaticPropertyValue",
		"getDefaultProperties",
		"isIterable",
		"isIterateable",
		"implementsInterface",
		"getExtension",
		"getExtensionName",
		"inNamespace",
		"getNamespaceName",
		"getShortName",
		"getAttributes",
	}
}

func getClassData(o *phpobj.ZObject) phpv.ZClass {
	v := o.GetOpaque(ReflectionClass)
	if v == nil {
		return nil
	}
	return v.(phpv.ZClass)
}

func getZClass(o *phpobj.ZObject) *phpobj.ZClass {
	c := getClassData(o)
	if c == nil {
		return nil
	}
	zc, ok := c.(*phpobj.ZClass)
	if !ok {
		return nil
	}
	return zc
}

// createReflectionClassObject creates a ReflectionClass (or ReflectionEnum) object for the given class,
// without going through __construct.
func createReflectionClassObject(ctx phpv.Context, class phpv.ZClass) (*phpv.ZVal, error) {
	// Use ReflectionEnum for enum classes
	targetClass := ReflectionClass
	if class.GetType().Has(phpv.ZClassTypeEnum) && ReflectionEnum != nil {
		targetClass = ReflectionEnum
	}
	obj, err := phpobj.CreateZObject(ctx, targetClass)
	if err != nil {
		return nil, err
	}
	obj.HashTable().SetString("name", class.GetName().ZVal())
	obj.SetOpaque(ReflectionClass, class)
	return obj.ZVal(), nil
}

func reflectionClassGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return class.GetName().ZVal(), nil
}

func reflectionClassGetParentClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	parent := class.GetParent()
	if phpv.IsNilClass(parent) {
		return phpv.ZBool(false).ZVal(), nil
	}
	return createReflectionClassObject(ctx, parent)
}

func reflectionClassGetInterfaceNames(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Use the same logic as getInterfaces but return only names
	arr := phpv.NewZArray()
	seen := make(map[string]bool)
	var collectInterfaceNames func(c *phpobj.ZClass)
	collectInterfaceNames = func(c *phpobj.ZClass) {
		for _, impl := range c.Implementations {
			key := strings.ToLower(string(impl.GetName()))
			if seen[key] {
				continue
			}
			seen[key] = true
			arr.OffsetSet(ctx, nil, impl.GetName().ZVal())
			collectInterfaceNames(impl)
		}
		parent := c.GetParent()
		if !phpv.IsNilClass(parent) {
			if pc, ok := parent.(*phpobj.ZClass); ok {
				if pc.Type == phpv.ZClassTypeInterface {
					key := strings.ToLower(string(pc.GetName()))
					if !seen[key] {
						seen[key] = true
						arr.OffsetSet(ctx, nil, pc.GetName().ZVal())
					}
					collectInterfaceNames(pc)
				} else {
					collectInterfaceNames(pc)
				}
			}
		}
	}
	collectInterfaceNames(zc)
	return arr.ZVal(), nil
}

func reflectionClassGetMethods(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Optional filter argument
	var filter int64 = -1 // -1 means no filter
	if len(args) > 0 && args[0].GetType() != phpv.ZtNull {
		filter = int64(args[0].AsInt(ctx))
	}

	// Use GetMethodsOrdered() for declaration-order iteration when available
	var methods []*phpv.ZClassMethod
	if zc, ok := class.(*phpobj.ZClass); ok {
		methods = zc.GetMethodsOrdered()
	} else {
		// Fallback for non-ZClass
		for _, method := range class.GetMethods() {
			methods = append(methods, method)
		}
	}

	arr := phpv.NewZArray()

	for _, method := range methods {
		// Skip private methods inherited from parent classes
		if method.Class != nil && method.Class.GetName() != class.GetName() && method.Modifiers.IsPrivate() {
			continue
		}

		if filter != -1 && !methodMatchesFilter(method, phpv.ZObjectAttr(filter)) {
			continue
		}
		// If this is a ReflectionObject, pass the underlying instance for closure __invoke support.
		var instance phpv.ZObject
		if opaque := o.GetOpaque(ReflectionObject); opaque != nil {
			instance, _ = opaque.(phpv.ZObject)
		}
		val, err := createReflectionMethodObjectWithInstance(ctx, class, method, instance)
		if err != nil {
			return nil, err
		}
		arr.OffsetSet(ctx, nil, val)
	}
	return arr.ZVal(), nil
}

func methodMatchesFilter(m *phpv.ZClassMethod, filter phpv.ZObjectAttr) bool {
	// Check each filter bit
	match := false

	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PUBLIC) != 0 {
		access := m.Modifiers.Access()
		if access == phpv.ZAttrPublic || access == 0 || m.Modifiers.Has(phpv.ZAttrImplicitPublic) {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PROTECTED) != 0 {
		if m.Modifiers.IsProtected() {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PRIVATE) != 0 {
		if m.Modifiers.IsPrivate() {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_STATIC) != 0 {
		if m.Modifiers.IsStatic() {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_ABSTRACT) != 0 {
		if m.Modifiers.Has(phpv.ZAttrAbstract) || m.Empty {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_FINAL) != 0 {
		if m.Modifiers.Has(phpv.ZAttrFinal) {
			match = true
		}
	}

	return match
}

// Filter constants (used as int64 values for class constants)
const (
	ReflectionMethodIS_STATIC    int64 = 16
	ReflectionMethodIS_ABSTRACT  int64 = 64
	ReflectionMethodIS_FINAL     int64 = 32
	ReflectionMethodIS_PUBLIC    int64 = 1
	ReflectionMethodIS_PROTECTED int64 = 2
	ReflectionMethodIS_PRIVATE   int64 = 4
)

func reflectionClassGetMethod(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getMethod() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::getMethod() expects exactly 1 argument, %d given", len(args)))
	}

	// Check argument type
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getMethod(): Argument #1 ($name) must be of type string, array given")
	}
	if args[0].GetType() == phpv.ZtObject {
		obj := args[0].AsObject(ctx)
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::getMethod(): Argument #1 ($name) must be of type string, %s given", obj.GetClass().GetName()))
	}

	class := getClassData(o)
	if class == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	methodName := args[0].AsString(ctx)
	method, ok := class.GetMethod(methodName)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Method %s::%s() does not exist", class.GetName(), methodName))
	}

	// If this is a ReflectionObject, pass the underlying instance so __invoke on closures works correctly.
	var instance phpv.ZObject
	if opaque := o.GetOpaque(ReflectionObject); opaque != nil {
		instance, _ = opaque.(phpv.ZObject)
	}
	return createReflectionMethodObjectWithInstance(ctx, class, method, instance)
}

func reflectionClassHasMethod(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::hasMethod() expects exactly 1 argument, 0 given")
	}

	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	methodName := args[0].AsString(ctx)
	_, ok := class.GetMethod(methodName)
	return phpv.ZBool(ok).ZVal(), nil
}

func reflectionClassGetProperties(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Optional filter argument
	var filter int64 = -1
	if len(args) > 0 && args[0].GetType() != phpv.ZtNull {
		filter = int64(args[0].AsInt(ctx))
	}

	arr := phpv.NewZArray()

	// Walk the class hierarchy to collect all properties
	seen := make(map[string]bool)
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			key := string(prop.VarName)
			if seen[key] {
				continue
			}
			// Private properties from parent classes are not visible
			if cur != zc && prop.Modifiers.IsPrivate() {
				continue
			}
			seen[key] = true

			if filter != -1 && !propertyMatchesFilter(prop, phpv.ZObjectAttr(filter)) {
				continue
			}

			// Use the actual declaring class for the property
			val, err := createReflectionPropertyObject(ctx, cur, prop)
			if err != nil {
				return nil, err
			}
			arr.OffsetSet(ctx, nil, val)
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		var ok bool
		cur, ok = parent.(*phpobj.ZClass)
		if !ok {
			break
		}
	}

	// For ReflectionObject, also include dynamic properties from the instance.
	// Use o.Class (not o.GetClass()) because when an inherited method is called,
	// the engine sets CurrentClass to the declaring class (ReflectionClass), but
	// o.Class always holds the actual concrete class of the instance.
	if o.Class == ReflectionObject {
		if instanceOpaque := o.GetOpaque(ReflectionObject); instanceOpaque != nil {
			if instance, ok := instanceOpaque.(phpv.ZObject); ok {
				// Use collectDynamicPropsForGetProps which excludes private parent props
				// from the "declared" set, so dynamic props with the same name as a
				// private parent prop ARE included (PHP getProperties() behavior).
				dynProps := collectDynamicPropsForGetProps(zc, instance)
				for _, dynName := range dynProps {
					if seen[string(dynName)] {
						continue
					}
					// Dynamic properties are always public
					// If filtering by visibility, only include if IS_PUBLIC is in the filter
					if filter != -1 {
						dynProp := &phpv.ZClassProp{
							VarName:   dynName,
							Modifiers: phpv.ZAttrPublic,
						}
						if !propertyMatchesFilter(dynProp, phpv.ZObjectAttr(filter)) {
							continue
						}
					}
					seen[string(dynName)] = true
					val, err := createReflectionPropertyObjectDynamic(ctx, zc, dynName)
					if err != nil {
						return nil, err
					}
					arr.OffsetSet(ctx, nil, val)
				}
			}
		}
	}

	return arr.ZVal(), nil
}

func propertyMatchesFilter(p *phpv.ZClassProp, filter phpv.ZObjectAttr) bool {
	match := false

	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PUBLIC) != 0 {
		access := p.Modifiers.Access()
		if access == phpv.ZAttrPublic || access == 0 {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PROTECTED) != 0 {
		if p.Modifiers.IsProtected() {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_PRIVATE) != 0 {
		if p.Modifiers.IsPrivate() {
			match = true
		}
	}
	if filter&phpv.ZObjectAttr(ReflectionMethodIS_STATIC) != 0 {
		if p.Modifiers.IsStatic() {
			match = true
		}
	}
	// IS_VIRTUAL = 512: property is virtual (has hooks but no backing store)
	if filter&512 != 0 {
		if p.HasHooks && !p.IsBacked {
			match = true
		}
	}
	// IS_READONLY = 128: property is readonly
	if filter&128 != 0 {
		if p.Modifiers.IsReadonly() {
			match = true
		}
	}

	return match
}

func reflectionClassHasProperty(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::hasProperty() expects exactly 1 argument, 0 given")
	}

	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	propName := args[0].AsString(ctx)

	// Walk the class hierarchy, skipping private properties from parent classes
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			if prop.VarName == propName {
				// Private properties from parent classes are not visible
				if cur != zc && prop.Modifiers.IsPrivate() {
					continue
				}
				return phpv.ZBool(true).ZVal(), nil
			}
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		var ok bool
		cur, ok = parent.(*phpobj.ZClass)
		if !ok {
			break
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionClassGetConstants(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Optional filter argument
	var filter int64 = -1
	if len(args) > 0 && args[0].GetType() != phpv.ZtNull {
		filter = int64(args[0].AsInt(ctx))
	}

	arr := phpv.NewZArray()
	// Collect constants from parent classes first, then override with child
	// Walk from the root parent to the current class so child constants win
	var classChain []*phpobj.ZClass
	for cls := zc; cls != nil; cls = cls.Extends {
		classChain = append(classChain, cls)
	}
	// Process from root parent to current class
	for i := len(classChain) - 1; i >= 0; i-- {
		cls := classChain[i]
		if cls.Const != nil {
			// Use ConstOrder if available, otherwise iterate the map
			names := cls.ConstOrder
			if len(names) == 0 {
				for n := range cls.Const {
					names = append(names, n)
				}
			}
			for _, name := range names {
				c := cls.Const[name]
				if c == nil || c.Value == nil {
					continue
				}
				// Private constants from parent classes are not visible on child classes.
				// When walking the chain for class B, skip private consts from A (cls != zc).
				// Also skip inherited copies of private constants (DeclaringClass set).
				if c.Modifiers.IsPrivate() && (cls != zc || c.DeclaringClass != nil) {
					continue
				}
				if filter != -1 && !classConstMatchesFilter(c, filter) {
					continue
				}
				val := c.Value
				if cd, ok := val.(*phpv.CompileDelayed); ok {
					resolved, err := cd.Run(ctx)
					if err != nil {
						continue
					}
					arr.OffsetSet(ctx, name, resolved)
				} else {
					arr.OffsetSet(ctx, name, val.ZVal())
				}
			}
		}
	}
	return arr.ZVal(), nil
}

func reflectionClassIsAbstract(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Only explicitly abstract classes return true
	// Interfaces are NOT considered abstract by ReflectionClass::isAbstract() in PHP 8.x
	isAbstract := zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0
	return phpv.ZBool(isAbstract).ZVal(), nil
}

func reflectionClassIsFinal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Attr.Has(phpv.ZClassFinal)).ZVal(), nil
}

func reflectionClassIsInterface(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Type == phpv.ZClassTypeInterface).ZVal(), nil
}

func reflectionClassIsInstantiable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Interfaces are not instantiable
	if zc.Type == phpv.ZClassTypeInterface {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Traits are not instantiable
	if zc.Type.Has(phpv.ZClassTypeTrait) {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Enums are not instantiable
	if zc.Type.Has(phpv.ZClassTypeEnum) {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Abstract classes are not instantiable
	if zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Check if constructor is public (or no constructor = instantiable)
	if m, ok := zc.GetMethod("__construct"); ok {
		if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	return phpv.ZBool(true).ZVal(), nil
}

func reflectionClassIsSubclassOf(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "ReflectionClass::isSubclassOf() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("ReflectionClass::isSubclassOf() expects exactly 1 argument, %d given", len(args)))
	}

	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var targetClass phpv.ZClass
	var err error

	if args[0].GetType() == phpv.ZtObject {
		// Could be a ReflectionClass object
		obj := args[0].AsObject(ctx)
		if obj != nil {
			opaque := obj.GetOpaque(ReflectionClass)
			if opaque != nil {
				targetClass = opaque.(phpv.ZClass)
			}
		}
	}

	if targetClass == nil {
		if args[0].GetType() == phpv.ZtNull {
			// PHP 8.1 deprecated passing null; emit deprecation and then fail with "Class "" does not exist"
			_ = ctx.Deprecated("ReflectionClass::isSubclassOf(): Passing null to parameter #1 ($class) of type ReflectionClass|string is deprecated", logopt.NoFuncName(true))
		}
		className := args[0].AsString(ctx)
		targetClass, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	}

	// isSubclassOf returns true if this class extends or implements the target,
	// but NOT if it IS the target
	if class.GetName() == targetClass.GetName() {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(class.InstanceOf(targetClass)).ZVal(), nil
}

func reflectionClassNewInstance(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// newInstance is declared with variadicArg("args"), so args[0] is a packed
	// ZArray containing the user-supplied constructor arguments. Unpack it.
	var constructArgs []*phpv.ZVal
	if len(args) > 0 {
		if arr, ok := args[0].Value().(*phpv.ZArray); ok {
			for _, v := range arr.Iterate(ctx) {
				constructArgs = append(constructArgs, v)
			}
		}
	}

	// Check if constructor exists and is accessible
	zc, _ := class.(*phpobj.ZClass)
	if zc != nil {
		var hasConstructor bool
		if zc.Handlers() != nil && zc.Handlers().Constructor != nil {
			ctorMethod := zc.Handlers().Constructor
			hasConstructor = true
			if ctorMethod.Modifiers.IsPrivate() || ctorMethod.Modifiers.IsProtected() {
				return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Access to non-public constructor of class %s", class.GetName()))
			}
		} else if m, ok := zc.GetMethod("__construct"); ok {
			hasConstructor = true
			if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
				return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Access to non-public constructor of class %s", class.GetName()))
			}
		}
		if !hasConstructor && len(constructArgs) > 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class %s does not have a constructor, so you cannot pass any constructor arguments", class.GetName()))
		}
	}

	// Suppress "called in X on line Y" in constructor errors - when called via
	// reflection, PHP does not include the call site in argument count errors.
	ctx.Global().SetNextCallSuppressCalledIn(true)
	obj, err := phpobj.NewZObject(ctx, class, constructArgs...)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func reflectionClassNewInstanceWithoutConstructor(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Enums cannot be instantiated
	if class.GetType().Has(phpv.ZClassTypeEnum) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot instantiate enum %s", class.GetName()))
	}

	// Internal-only final classes (e.g. Generator) cannot be instantiated without constructor
	if zc, ok := class.(*phpobj.ZClass); ok && zc.InternalOnly && zc.Attr.Has(phpv.ZClassFinal) {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class %s is an internal class marked as final that cannot be instantiated without invoking its constructor", class.GetName()))
	}

	obj, err := phpobj.CreateZObject(ctx, class)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func reflectionClassGetConstructor(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Check handlers constructor first
	if class.Handlers() != nil && class.Handlers().Constructor != nil {
		return createReflectionMethodObject(ctx, class, class.Handlers().Constructor)
	}

	method, ok := class.GetMethod("__construct")
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionMethodObject(ctx, class, method)
}

// createReflectionPropertyObject creates a ReflectionProperty object for the given
// class and property.
func createReflectionPropertyObject(ctx phpv.Context, class *phpobj.ZClass, prop *phpv.ZClassProp) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionProperty)
	if err != nil {
		return nil, err
	}
	data := &reflectionPropertyData{
		prop:  prop,
		class: class,
	}
	obj.HashTable().SetString("name", prop.VarName.ZVal())
	obj.HashTable().SetString("class", class.GetName().ZVal())
	obj.SetOpaque(ReflectionProperty, data)
	return obj.ZVal(), nil
}

// createReflectionPropertyObjectDynamic creates a ReflectionProperty object for a dynamic
// (runtime-added) property that is not declared in the class definition.
func createReflectionPropertyObjectDynamic(ctx phpv.Context, class *phpobj.ZClass, propName phpv.ZString) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionProperty)
	if err != nil {
		return nil, err
	}
	prop := &phpv.ZClassProp{
		VarName:   propName,
		Modifiers: phpv.ZAttrPublic,
	}
	data := &reflectionPropertyData{
		prop:      prop,
		class:     class,
		isDynamic: true,
	}
	obj.HashTable().SetString("name", propName.ZVal())
	obj.HashTable().SetString("class", class.GetName().ZVal())
	obj.SetOpaque(ReflectionProperty, data)
	return obj.ZVal(), nil
}

// reflectionClassGetProperty handles both plain property names and ClassName::propName syntax
func reflectionClassGetProperty(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getProperty() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::getProperty() expects exactly 1 argument, %d given", len(args)))
	}
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getProperty(): Argument #1 ($name) must be of type string, array given")
	}
	if args[0].GetType() == phpv.ZtObject {
		obj := args[0].AsObject(ctx)
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::getProperty(): Argument #1 ($name) must be of type string, %s given", obj.GetClass().GetName()))
	}
	name := args[0].AsString(ctx)

	zc := getZClass(o)
	if zc == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s does not exist", name))
	}

	// Check if the name contains "::" (class::property syntax)
	if idx := strings.Index(string(name), "::"); idx != -1 {
		className := phpv.ZString(name[:idx])
		propName := phpv.ZString(name[idx+2:])
		// Strip leading $ if present
		if len(propName) > 0 && propName[0] == '$' {
			propName = propName[1:]
		}

		// Resolve the specified class (use lowercase for error messages, PHP normalizes class names)
		specClass, err := resolveClass(ctx, className.ToLower())
		if err != nil {
			return nil, err
		}

		// Check that the specified class is in the hierarchy of the reflected class
		specZc, ok := specClass.(*phpobj.ZClass)
		if !ok {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", specClass.GetName(), propName))
		}

		// Check if specClass is the same or a parent of the reflected class
		isInHierarchy := false
		for cur := zc; cur != nil; {
			if cur.GetName().ToLower() == specZc.GetName().ToLower() {
				isInHierarchy = true
				break
			}
			parent := cur.GetParent()
			if phpv.IsNilClass(parent) {
				break
			}
			var ok2 bool
			cur, ok2 = parent.(*phpobj.ZClass)
			if !ok2 {
				break
			}
		}

		if !isInHierarchy {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Fully qualified property name %s::$%s does not specify a base class of %s", specClass.GetName(), propName, zc.GetName()))
		}

		// Look for the property in the specified class
		for _, prop := range specZc.Props {
			if prop.VarName == propName {
				return createReflectionPropertyObject(ctx, specZc, prop)
			}
		}
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", specClass.GetName(), propName))
	}

	// Walk the class hierarchy, skipping private properties from parent classes
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			if prop.VarName == name {
				if cur != zc && prop.Modifiers.IsPrivate() {
					continue
				}
				return createReflectionPropertyObject(ctx, cur, prop)
			}
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		var ok bool
		cur, ok = parent.(*phpobj.ZClass)
		if !ok {
			break
		}
	}

	// For ReflectionObject, also check dynamic properties on the instance
	if o.Class == ReflectionObject {
		if instanceOpaque := o.GetOpaque(ReflectionObject); instanceOpaque != nil {
			if instance, ok := instanceOpaque.(phpv.ZObject); ok {
				if zobj, ok2 := instance.(*phpobj.ZObject); ok2 {
					if zobj.HashTable().HasString(name) {
						return createReflectionPropertyObjectDynamic(ctx, zc, name)
					}
				}
			}
		}
	}

	return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", zc.GetName(), name))
}

func reflectionClassGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, zc.Attributes, phpobj.AttributeTARGET_CLASS, name, flags, zc)
}

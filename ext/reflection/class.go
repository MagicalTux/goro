package reflection

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func initReflectionClass() {
	// ReflectionClass is declared in ext.go; we extend its methods here
	ReflectionClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":              {Name: "__construct", Method: phpobj.NativeMethod(reflectionClassConstruct)},
		"getname":                  {Name: "getName", Method: phpobj.NativeMethod(reflectionClassGetName)},
		"getparentclass":           {Name: "getParentClass", Method: phpobj.NativeMethod(reflectionClassGetParentClass)},
		"getinterfacenames":        {Name: "getInterfaceNames", Method: phpobj.NativeMethod(reflectionClassGetInterfaceNames)},
		"getmethods":              {Name: "getMethods", Method: phpobj.NativeMethod(reflectionClassGetMethods)},
		"getmethod":               {Name: "getMethod", Method: phpobj.NativeMethod(reflectionClassGetMethod)},
		"hasmethod":               {Name: "hasMethod", Method: phpobj.NativeMethod(reflectionClassHasMethod)},
		"getproperties":           {Name: "getProperties", Method: phpobj.NativeMethod(reflectionClassGetProperties)},
		"getproperty":             {Name: "getProperty", Method: phpobj.NativeMethod(reflectionClassGetProperty)},
		"hasproperty":             {Name: "hasProperty", Method: phpobj.NativeMethod(reflectionClassHasProperty)},
		"getconstants":            {Name: "getConstants", Method: phpobj.NativeMethod(reflectionClassGetConstants)},
		"isabstract":              {Name: "isAbstract", Method: phpobj.NativeMethod(reflectionClassIsAbstract)},
		"isfinal":                 {Name: "isFinal", Method: phpobj.NativeMethod(reflectionClassIsFinal)},
		"isinterface":             {Name: "isInterface", Method: phpobj.NativeMethod(reflectionClassIsInterface)},
		"isinstantiable":          {Name: "isInstantiable", Method: phpobj.NativeMethod(reflectionClassIsInstantiable)},
		"issubclassof":            {Name: "isSubclassOf", Method: phpobj.NativeMethod(reflectionClassIsSubclassOf)},
		"newinstance":             {Name: "newInstance", Method: phpobj.NativeMethod(reflectionClassNewInstance)},
		"newinstancewithoutconstructor": {Name: "newInstanceWithoutConstructor", Method: phpobj.NativeMethod(reflectionClassNewInstanceWithoutConstructor)},
		"getconstructor":          {Name: "getConstructor", Method: phpobj.NativeMethod(reflectionClassGetConstructor)},
		"implementsinterface":     {Name: "implementsInterface", Method: phpobj.NativeMethod(reflectionClassImplementsInterface)},
		"getattributes":           {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionClassGetAttributes)},
		"getreflectionconstant":   {Name: "getReflectionConstant", Method: phpobj.NativeMethod(reflectionClassGetReflectionConstant)},
		"getreflectionconstants":  {Name: "getReflectionConstants", Method: phpobj.NativeMethod(reflectionClassGetReflectionConstants)},
		"getdoccomment":           {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionClassGetDocComment)},
		"__tostring":              {Name: "__toString", Method: phpobj.NativeMethod(reflectionClassToString)},
		"getshortname":            {Name: "getShortName", Method: phpobj.NativeMethod(reflectionClassGetShortName)},
		"getnamespacename":        {Name: "getNamespaceName", Method: phpobj.NativeMethod(reflectionClassGetNamespaceName)},
		"innamespace":             {Name: "inNamespace", Method: phpobj.NativeMethod(reflectionClassInNamespace)},
		"getinterfaces":           {Name: "getInterfaces", Method: phpobj.NativeMethod(reflectionClassGetInterfaces)},
		"hasconstant":             {Name: "hasConstant", Method: phpobj.NativeMethod(reflectionClassHasConstant)},
		"getconstant":             {Name: "getConstant", Method: phpobj.NativeMethod(reflectionClassGetConstant)},
		"getdefaultproperties":    {Name: "getDefaultProperties", Method: phpobj.NativeMethod(reflectionClassGetDefaultProperties)},
		"getstaticproperties":     {Name: "getStaticProperties", Method: phpobj.NativeMethod(reflectionClassGetStaticProperties)},
		"getstaticpropertyvalue":  {Name: "getStaticPropertyValue", Method: phpobj.NativeMethod(reflectionClassGetStaticPropertyValue)},
		"setstaticpropertyvalue":  {Name: "setStaticPropertyValue", Method: phpobj.NativeMethod(reflectionClassSetStaticPropertyValue)},
		"newinstanceargs":         {Name: "newInstanceArgs", Method: phpobj.NativeMethod(reflectionClassNewInstanceArgs)},
		"iscloneable":             {Name: "isCloneable", Method: phpobj.NativeMethod(reflectionClassIsCloneable)},
		"isanonymous":             {Name: "isAnonymous", Method: phpobj.NativeMethod(reflectionClassIsAnonymous)},
		"isenum":                  {Name: "isEnum", Method: phpobj.NativeMethod(reflectionClassIsEnum)},
		"istrait":                 {Name: "isTrait", Method: phpobj.NativeMethod(reflectionClassIsTrait)},
		"isreadonly":              {Name: "isReadOnly", Method: phpobj.NativeMethod(reflectionClassIsReadOnly)},
		"isiterable":              {Name: "isIterable", Method: phpobj.NativeMethod(reflectionClassIsIterable)},
		"isiterateable":           {Name: "isIterateable", Method: phpobj.NativeMethod(reflectionClassIsIterable)},
		"isinstance":              {Name: "isInstance", Method: phpobj.NativeMethod(reflectionClassIsInstance)},
		"isinternal":              {Name: "isInternal", Method: phpobj.NativeMethod(reflectionClassIsInternal)},
		"isuserdefined":           {Name: "isUserDefined", Method: phpobj.NativeMethod(reflectionClassIsUserDefined)},
		"getfilename":             {Name: "getFileName", Method: phpobj.NativeMethod(reflectionClassGetFileName)},
		"getstartline":            {Name: "getStartLine", Method: phpobj.NativeMethod(reflectionClassGetStartLine)},
		"getendline":              {Name: "getEndLine", Method: phpobj.NativeMethod(reflectionClassGetEndLine)},
		"getmodifiers":            {Name: "getModifiers", Method: phpobj.NativeMethod(reflectionClassGetModifiers)},
		"getextension":            {Name: "getExtension", Method: phpobj.NativeMethod(reflectionClassGetExtension)},
		"getextensionname":        {Name: "getExtensionName", Method: phpobj.NativeMethod(reflectionClassGetExtensionName)},
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

// createReflectionClassObject creates a ReflectionClass object for the given class,
// without going through __construct.
func createReflectionClassObject(ctx phpv.Context, class phpv.ZClass) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionClass)
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

	arr := phpv.NewZArray()
	// Collect interface names from Implementations
	for _, impl := range zc.Implementations {
		arr.OffsetSet(ctx, nil, impl.GetName().ZVal())
	}
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

	methods := class.GetMethods()
	arr := phpv.NewZArray()

	for _, method := range methods {
		if filter != -1 && !methodMatchesFilter(method, phpv.ZObjectAttr(filter)) {
			continue
		}
		val, err := createReflectionMethodObject(ctx, class, method)
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

	// If no access/modifier bits were set in the filter, it means show all
	if filter == 0 {
		return true
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::getMethod() expects exactly 1 argument, 0 given")
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

	return createReflectionMethodObject(ctx, class, method)
}

func reflectionClassHasMethod(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::hasMethod() expects exactly 1 argument, 0 given")
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
			seen[key] = true

			if filter != -1 && !propertyMatchesFilter(prop, phpv.ZObjectAttr(filter)) {
				continue
			}

			val, err := createReflectionPropertyObject(ctx, zc, prop)
			if err != nil {
				return nil, err
			}
			arr.OffsetSet(ctx, nil, val)
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		cur = parent.(*phpobj.ZClass)
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

	if filter == 0 {
		return true
	}

	return match
}

func reflectionClassHasProperty(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::hasProperty() expects exactly 1 argument, 0 given")
	}

	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	propName := args[0].AsString(ctx)
	_, found := class.GetProp(propName)
	return phpv.ZBool(found).ZVal(), nil
}

func reflectionClassGetConstants(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	arr := phpv.NewZArray()
	if zc.Const != nil {
		for _, name := range zc.ConstOrder {
			if c := zc.Const[name]; c != nil && c.Value != nil {
				arr.OffsetSet(ctx, name, c.Value.ZVal())
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
	// Check for explicit abstract or interface (interfaces are implicitly abstract)
	isAbstract := zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 || zc.Type == phpv.ZClassTypeInterface
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::isSubclassOf() expects exactly 1 argument, 0 given")
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

	obj, err := phpobj.NewZObject(ctx, class, args...)
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

// Override the original reflectionClassGetProperty to use the new property lookup
func reflectionClassGetProperty(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::getProperty() expects exactly 1 argument, 0 given")
	}
	name := args[0].AsString(ctx)

	// Check if the name contains "::" (class::property syntax)
	if idx := strings.Index(string(name), "::"); idx != -1 {
		className := phpv.ZString(strings.ToLower(string(name[:idx])))
		_, err := resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	}

	zc := getZClass(o)
	if zc == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s does not exist", name))
	}

	prop, found := zc.GetProp(name)
	if !found {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", zc.GetName(), name))
	}

	return createReflectionPropertyObject(ctx, zc, prop)
}

func reflectionClassGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, zc.Attributes, phpobj.AttributeTARGET_CLASS, name, flags)
}

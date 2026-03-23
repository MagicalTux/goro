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
		"gettraits":               {Name: "getTraits", Method: phpobj.NativeMethod(reflectionClassGetTraits)},
		"gettraitnames":           {Name: "getTraitNames", Method: phpobj.NativeMethod(reflectionClassGetTraitNames)},
		"gettraitaliases":         {Name: "getTraitAliases", Method: phpobj.NativeMethod(reflectionClassGetTraitAliases)},
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

	if args[0].GetType() == phpv.ZtNull {
		_ = ctx.Deprecated("Passing null to parameter #1 ($name) of type string is deprecated")
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
	if zc.Const != nil {
		for _, name := range zc.ConstOrder {
			c := zc.Const[name]
			if c == nil || c.Value == nil {
				continue
			}
			if filter != -1 && !classConstMatchesFilter(c, filter) {
				continue
			}
			// Resolve CompileDelayed values
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
			_ = ctx.Deprecated("Passing null to parameter #1 ($class) of type ReflectionClass|string is deprecated")
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

	// Check if constructor exists and is accessible
	zc, _ := class.(*phpobj.ZClass)
	if zc != nil {
		var hasConstructor bool
		if zc.Handlers() != nil && zc.Handlers().Constructor != nil {
			hasConstructor = true
		} else if m, ok := zc.GetMethod("__construct"); ok {
			hasConstructor = true
			if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
				return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Access to non-public constructor of class %s", class.GetName()))
			}
		}
		if !hasConstructor && len(args) > 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class %s does not have a constructor, so you cannot pass any constructor arguments", class.GetName()))
		}
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

	// Enums cannot be instantiated
	if class.GetType().Has(phpv.ZClassTypeEnum) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot instantiate enum %s", class.GetName()))
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
	if args[0].GetType() == phpv.ZtNull {
		_ = ctx.Deprecated("Passing null to parameter #1 ($name) of type string is deprecated")
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

		// Resolve the specified class
		specClass, err := resolveClass(ctx, className)
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

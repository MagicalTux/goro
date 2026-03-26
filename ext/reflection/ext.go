package reflection

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ReflectionException *phpobj.ZClass

var PropertyHookType *phpobj.ZClass

var ReflectionClass *phpobj.ZClass
var ReflectionClassConstant *phpobj.ZClass
var ReflectionConstant *phpobj.ZClass
var ReflectionEnum *phpobj.ZClass
var ReflectionEnumBackedCase *phpobj.ZClass
var ReflectionEnumUnitCase *phpobj.ZClass
var ReflectionMethod *phpobj.ZClass
var ReflectionObject *phpobj.ZClass
var ReflectionProperty *phpobj.ZClass

func init() {
	// PropertyHookType is a string-backed enum with cases Get and Set (PHP 8.4+)
	PropertyHookType = &phpobj.ZClass{
		Name: "PropertyHookType",
		Type: phpv.ZClassTypeEnum,
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"Get": {Value: &phpv.CompileDelayed{V: &builtinEnumCaseInit{caseName: "Get", backingValue: "get"}}, Modifiers: phpv.ZAttrPublic},
			"Set": {Value: &phpv.CompileDelayed{V: &builtinEnumCaseInit{caseName: "Set", backingValue: "set"}}, Modifiers: phpv.ZAttrPublic},
		},
		ConstOrder:      []phpv.ZString{"Get", "Set"},
		EnumBackingType: phpv.ZtString,
		EnumCases:       []phpv.ZString{"Get", "Set"},
		Methods:         map[phpv.ZString]*phpv.ZClassMethod{},
	}
	// Set the enum class reference on the const initializers (needs to be done after PropertyHookType is created)
	PropertyHookType.Const["Get"].Value.(*phpv.CompileDelayed).V.(*builtinEnumCaseInit).enumClass = PropertyHookType
	PropertyHookType.Const["Set"].Value.(*phpv.CompileDelayed).V.(*builtinEnumCaseInit).enumClass = PropertyHookType

	ReflectionException = &phpobj.ZClass{
		Name:    "ReflectionException",
		Extends: phpobj.Exception,
		Props:   phpobj.Exception.Props,
		Methods: phpobj.CopyMethods(phpobj.Exception.Methods),
	}

	ReflectionClass = &phpobj.ZClass{
		Name: "ReflectionClass",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic, TypeHint: phpv.ParseTypeHint("string")},
		},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_IMPLICIT_ABSTRACT":            {Value: phpv.ZInt(16)},
			"IS_EXPLICIT_ABSTRACT":            {Value: phpv.ZInt(64)},
			"IS_FINAL":                        {Value: phpv.ZInt(32)},
			"IS_READONLY":                     {Value: phpv.ZInt(65536)},
			"SKIP_INITIALIZATION_ON_SERIALIZE": {Value: phpv.ZInt(8)},
			"SKIP_DESTRUCTOR":                 {Value: phpv.ZInt(16)},
		},
		// Methods will be set by initReflectionClass()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionMethod = &phpobj.ZClass{
		Name: "ReflectionMethod",
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_STATIC":    {Value: phpv.ZInt(ReflectionMethodIS_STATIC)},
			"IS_ABSTRACT":  {Value: phpv.ZInt(ReflectionMethodIS_ABSTRACT)},
			"IS_FINAL":     {Value: phpv.ZInt(ReflectionMethodIS_FINAL)},
			"IS_PUBLIC":    {Value: phpv.ZInt(ReflectionMethodIS_PUBLIC)},
			"IS_PROTECTED": {Value: phpv.ZInt(ReflectionMethodIS_PROTECTED)},
			"IS_PRIVATE":   {Value: phpv.ZInt(ReflectionMethodIS_PRIVATE)},
		},
		// Methods will be set by initReflectionMethod()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionProperty = &phpobj.ZClass{
		Name: "ReflectionProperty",
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_STATIC":        {Value: phpv.ZInt(ReflectionMethodIS_STATIC)},
			"IS_PUBLIC":        {Value: phpv.ZInt(ReflectionMethodIS_PUBLIC)},
			"IS_PROTECTED":     {Value: phpv.ZInt(ReflectionMethodIS_PROTECTED)},
			"IS_PRIVATE":       {Value: phpv.ZInt(ReflectionMethodIS_PRIVATE)},
			"IS_READONLY":      {Value: phpv.ZInt(128)},
			"IS_VIRTUAL":       {Value: phpv.ZInt(512)},
			"IS_PROTECTED_SET": {Value: phpv.ZInt(256)},
			"IS_PRIVATE_SET":   {Value: phpv.ZInt(1024)},
			"IS_ABSTRACT":      {Value: phpv.ZInt(ReflectionMethodIS_ABSTRACT)},
			"IS_FINAL":         {Value: phpv.ZInt(ReflectionMethodIS_FINAL)},
		},
		// Methods will be set by initReflectionProperty()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionClassConstant = &phpobj.ZClass{
		Name: "ReflectionClassConstant",
		// Const, Props, and Methods will be set by initReflectionClassConstant()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionConstant = &phpobj.ZClass{
		Name: "ReflectionConstant",
		// Methods will be set by initReflectionConstant()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionEnum = &phpobj.ZClass{
		Name:    "ReflectionEnum",
		Extends: ReflectionClass,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	ReflectionEnumBackedCase = &phpobj.ZClass{
		Name: "ReflectionEnumBackedCase",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil // stub
			})},
		},
	}

	ReflectionEnumUnitCase = &phpobj.ZClass{
		Name: "ReflectionEnumUnitCase",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil // stub
			})},
		},
	}

	// Initialize sub-class definitions (types, parameters, etc.)
	initReflectionType()
	initReflectionParameter()
	initReflectionFunction()
	initReflectionAttribute()
	initReflectionExtension()
	initReflectionStatic()
	initReflectionGenerator()

	// Initialize methods on pre-declared classes
	initReflectionClass()
	initReflectionMethod()
	initReflectionProperty()
	initReflectionClassConstant()
	initReflectionConstant()

	// Initialize lazy object methods on ReflectionClass and ReflectionProperty
	initLazyObjectMethods()

	// Copy methods to classes that inherit from ReflectionClass (after it's fully initialized)
	ReflectionEnum.Methods = phpobj.CopyMethods(ReflectionClass.Methods)
	ReflectionEnum.Props = ReflectionClass.Props
	initReflectionEnum()

	// ReflectionObject extends ReflectionClass with the same behavior
	ReflectionObject = &phpobj.ZClass{
		Name:    "ReflectionObject",
		Extends: ReflectionClass,
		Props:   ReflectionClass.Props,
		Methods: phpobj.CopyMethods(ReflectionClass.Methods),
	}

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "Reflection",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			PropertyHookType,
			ReflectionException,
			ReflectionClass,
			ReflectionClassConstant,
			ReflectionConstant,
			ReflectionEnum,
			ReflectionEnumBackedCase,
			ReflectionEnumUnitCase,
			ReflectionExtension,
			ReflectionStatic,
			ReflectionMethod,
			ReflectionObject,
			ReflectionProperty,
			ReflectionFunction,
			ReflectionParameter,
			ReflectionType,
			ReflectionNamedType,
			ReflectionUnionType,
			ReflectionIntersectionType,
			ReflectionAttribute,
			ReflectionGenerator,
			ReflectionFiber,
			ReflectionReference,
		},
	})
}

// builtinEnumCaseInit is a Runnable that lazily creates a built-in enum case object.
type builtinEnumCaseInit struct {
	enumClass    *phpobj.ZClass
	caseName     phpv.ZString
	backingValue phpv.ZString
}

func (b *builtinEnumCaseInit) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", b.enumClass.Name, b.caseName)
	return err
}

func (b *builtinEnumCaseInit) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	obj := phpobj.NewZObjectEnum(ctx, b.enumClass)
	obj.HashTable().SetString("name", b.caseName.ZVal())
	obj.HashTable().SetString("value", b.backingValue.ZVal())
	return obj.ZVal(), nil
}

// resolveClass tries to find a class by name, triggering autoload.
// Returns the class or throws ReflectionException if not found.
func resolveClass(ctx phpv.Context, className phpv.ZString) (phpv.ZClass, error) {
	class, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" does not exist", className))
	}
	return class, nil
}

func reflectionClassConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::__construct() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::__construct() expects exactly 1 argument, %d given", len(args)))
	}
	arg := args[0]
	var class phpv.ZClass
	if arg.GetType() == phpv.ZtObject {
		// For objects, use the class directly (handles anonymous classes)
		class = arg.AsObject(ctx).GetClass()
	} else if arg.GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::__construct(): Argument #1 ($objectOrClass) must be of type object|string, array given")
	} else {
		if arg.GetType() == phpv.ZtNull {
			_ = ctx.Deprecated("Passing null to parameter #1 ($objectOrClass) of type object|string is deprecated")
		}
		className := arg.AsString(ctx)
		var err error
		class, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	}

	o.HashTable().SetString("name", class.GetName().ZVal())
	o.SetOpaque(ReflectionClass, class)
	return nil, nil
}

func reflectionClassImplementsInterface(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "ReflectionClass::implementsInterface() expects exactly 1 argument, 0 given")
	}
	if len(args) > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("ReflectionClass::implementsInterface() expects exactly 1 argument, %d given", len(args)))
	}

	var iface phpv.ZClass
	var err error

	if args[0].GetType() == phpv.ZtObject {
		// Could be a ReflectionClass object
		obj := args[0].AsObject(ctx)
		if obj != nil {
			opaque := obj.GetOpaque(ReflectionClass)
			if opaque != nil {
				iface = opaque.(phpv.ZClass)
			}
		}
	}

	if iface == nil {
		ifaceName := args[0].AsString(ctx)
		iface, err = ctx.Global().GetClass(ctx, ifaceName, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Interface \"%s\" does not exist", ifaceName))
		}
	}

	// Check that the argument is actually an interface
	if iface.GetType() != phpv.ZClassTypeInterface {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("%s is not an interface", iface.GetName()))
	}

	class := o.GetOpaque(ReflectionClass).(phpv.ZClass)
	return phpv.ZBool(class.InstanceOf(iface)).ZVal(), nil
}

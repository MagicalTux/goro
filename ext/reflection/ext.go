package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ReflectionException *phpobj.ZClass

var ReflectionClass *phpobj.ZClass
var ReflectionMethod *phpobj.ZClass
var ReflectionProperty *phpobj.ZClass

func init() {
	ReflectionException = &phpobj.ZClass{
		Name:    "ReflectionException",
		Extends: phpobj.Exception,
		Props:   phpobj.Exception.Props,
		Methods: phpobj.CopyMethods(phpobj.Exception.Methods),
	}

	ReflectionClass = &phpobj.ZClass{
		Name: "ReflectionClass",
		Props: []*phpv.ZClassProp{
			{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_IMPLICIT_ABSTRACT": {Value: phpv.ZInt(16)},
			"IS_EXPLICIT_ABSTRACT": {Value: phpv.ZInt(64)},
			"IS_FINAL":             {Value: phpv.ZInt(32)},
			"IS_READONLY":          {Value: phpv.ZInt(65536)},
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
			"IS_STATIC":    {Value: phpv.ZInt(ReflectionMethodIS_STATIC)},
			"IS_PUBLIC":    {Value: phpv.ZInt(ReflectionMethodIS_PUBLIC)},
			"IS_PROTECTED": {Value: phpv.ZInt(ReflectionMethodIS_PROTECTED)},
			"IS_PRIVATE":   {Value: phpv.ZInt(ReflectionMethodIS_PRIVATE)},
			"IS_READONLY":  {Value: phpv.ZInt(128)},
		},
		// Methods will be set by initReflectionProperty()
		Methods: map[phpv.ZString]*phpv.ZClassMethod{},
	}

	// Initialize sub-class definitions (types, parameters, etc.)
	initReflectionType()
	initReflectionParameter()
	initReflectionFunction()

	// Initialize methods on pre-declared classes
	initReflectionClass()
	initReflectionMethod()
	initReflectionProperty()

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "Reflection",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			ReflectionException,
			ReflectionClass,
			ReflectionMethod,
			ReflectionProperty,
			ReflectionFunction,
			ReflectionParameter,
			ReflectionType,
			ReflectionNamedType,
		},
	})
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::__construct() expects exactly 1 argument, 0 given")
	}
	arg := args[0]
	var className phpv.ZString
	if arg.GetType() == phpv.ZtObject {
		className = arg.AsObject(ctx).GetClass().GetName()
	} else {
		className = arg.AsString(ctx)
	}

	class, err := resolveClass(ctx, className)
	if err != nil {
		return nil, err
	}

	o.HashTable().SetString("name", class.GetName().ZVal())
	o.SetOpaque(ReflectionClass, class)
	return nil, nil
}

func reflectionClassImplementsInterface(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::implementsInterface() expects exactly 1 argument, 0 given")
	}
	ifaceName := args[0].AsString(ctx)

	// Try to resolve the interface (triggers autoload)
	iface, err := ctx.Global().GetClass(ctx, ifaceName, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Interface \"%s\" does not exist", ifaceName))
	}

	class := o.GetOpaque(ReflectionClass).(phpv.ZClass)
	return phpv.ZBool(class.InstanceOf(iface)).ZVal(), nil
}

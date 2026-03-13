package reflection

import (
	"fmt"
	"strings"

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
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(reflectionClassConstruct)},
			"getproperty": {Name: "getProperty", Method: phpobj.NativeMethod(reflectionClassGetProperty)},
			"implementsinterface": {Name: "implementsInterface", Method: phpobj.NativeMethod(reflectionClassImplementsInterface)},
		},
	}

	ReflectionMethod = &phpobj.ZClass{
		Name: "ReflectionMethod",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(reflectionMethodConstruct)},
		},
	}

	ReflectionProperty = &phpobj.ZClass{
		Name: "ReflectionProperty",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(reflectionPropertyConstruct)},
		},
	}

	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "Reflection",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			ReflectionException,
			ReflectionClass,
			ReflectionMethod,
			ReflectionProperty,
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

	// For now, throw ReflectionException for any property not found
	return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", o.HashTable().GetString("name").AsString(ctx), name))
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

func reflectionMethodConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::__construct() expects exactly 2 arguments")
	}
	className := args[0].AsString(ctx)
	_, err := resolveClass(ctx, className)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func reflectionPropertyConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::__construct() expects exactly 2 arguments")
	}
	className := args[0].AsString(ctx)
	_, err := resolveClass(ctx, className)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

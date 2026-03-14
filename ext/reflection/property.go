package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// reflectionPropertyData is stored as opaque data on ReflectionProperty objects
type reflectionPropertyData struct {
	prop  *phpv.ZClassProp
	class *phpobj.ZClass
}

func initReflectionProperty() {
	// ReflectionProperty is declared in ext.go; we extend its methods here
	ReflectionProperty.Props = []*phpv.ZClassProp{
		{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		{VarName: "class", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
	}
	ReflectionProperty.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {Name: "__construct", Method: phpobj.NativeMethod(reflectionPropertyConstructFull)},
		"getname":     {Name: "getName", Method: phpobj.NativeMethod(reflectionPropertyGetName)},
		"ispublic":    {Name: "isPublic", Method: phpobj.NativeMethod(reflectionPropertyIsPublic)},
		"isprotected": {Name: "isProtected", Method: phpobj.NativeMethod(reflectionPropertyIsProtected)},
		"isprivate":   {Name: "isPrivate", Method: phpobj.NativeMethod(reflectionPropertyIsPrivate)},
		"isstatic":    {Name: "isStatic", Method: phpobj.NativeMethod(reflectionPropertyIsStatic)},
		"isdefault":   {Name: "isDefault", Method: phpobj.NativeMethod(reflectionPropertyIsDefault)},
		"getvalue":    {Name: "getValue", Method: phpobj.NativeMethod(reflectionPropertyGetValue)},
		"setvalue":    {Name: "setValue", Method: phpobj.NativeMethod(reflectionPropertySetValue)},
		"getdeclaringclass": {Name: "getDeclaringClass", Method: phpobj.NativeMethod(reflectionPropertyGetDeclaringClass)},
		"getattributes":     {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionPropertyGetAttributes)},
	}
}

func reflectionPropertyConstructFull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::__construct() expects exactly 2 arguments")
	}

	var class phpv.ZClass
	var err error

	if args[0].GetType() == phpv.ZtObject {
		class = args[0].AsObject(ctx).GetClass()
	} else {
		className := args[0].AsString(ctx)
		class, err = resolveClass(ctx, className)
		if err != nil {
			return nil, err
		}
	}

	propName := args[1].AsString(ctx)
	prop, found := class.GetProp(propName)
	if !found {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", class.GetName(), propName))
	}

	zc, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: unexpected class type")
	}

	data := &reflectionPropertyData{
		prop:  prop,
		class: zc,
	}
	o.HashTable().SetString("name", prop.VarName.ZVal())
	o.HashTable().SetString("class", class.GetName().ZVal())
	o.SetOpaque(ReflectionProperty, data)
	return nil, nil
}

func getPropData(o *phpobj.ZObject) *reflectionPropertyData {
	v := o.GetOpaque(ReflectionProperty)
	if v == nil {
		return nil
	}
	return v.(*reflectionPropertyData)
}

func reflectionPropertyGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.prop.VarName.ZVal(), nil
}

func reflectionPropertyIsPublic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	access := data.prop.Modifiers.Access()
	return phpv.ZBool(access == phpv.ZAttrPublic || access == 0).ZVal(), nil
}

func reflectionPropertyIsProtected(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsProtected()).ZVal(), nil
}

func reflectionPropertyIsPrivate(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsPrivate()).ZVal(), nil
}

func reflectionPropertyIsStatic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsStatic()).ZVal(), nil
}

func reflectionPropertyIsDefault(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// isDefault returns true if property was declared at compile time (in class definition)
	// as opposed to dynamically added at runtime. Since all properties we reflect on
	// come from ZClassProp, they are all declared properties.
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionPropertyGetValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// For static properties
	if data.prop.Modifiers.IsStatic() {
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		v := staticProps.GetString(data.prop.VarName)
		if v != nil {
			return v, nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	// For instance properties, need an object argument
	if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionProperty::getValue(): argument must be an object for non-static properties")
	}

	obj := args[0].AsObject(ctx)
	return obj.ObjectGet(ctx, data.prop.VarName)
}

func reflectionPropertySetValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// For static properties
	if data.prop.Modifiers.IsStatic() {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setValue() expects at least 1 argument for static properties")
		}
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		return nil, staticProps.SetString(data.prop.VarName, args[0])
	}

	// For instance properties
	if len(args) < 2 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setValue() expects an object and a value for non-static properties")
	}

	obj := args[0].AsObject(ctx)
	return nil, obj.ObjectSet(ctx, data.prop.VarName, args[1])
}

func reflectionPropertyGetDeclaringClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionClassObject(ctx, data.class)
}

func reflectionPropertyGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.prop.Attributes, phpobj.AttributeTARGET_PROPERTY, name, flags)
}

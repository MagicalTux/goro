package reflection

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
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
		"getdoccomment":     {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionPropertyGetDocComment)},
		"isreadonly":        {Name: "isReadOnly", Method: phpobj.NativeMethod(reflectionPropertyIsReadOnly)},
		"gettype":           {Name: "getType", Method: phpobj.NativeMethod(reflectionPropertyGetType)},
		"hastype":           {Name: "hasType", Method: phpobj.NativeMethod(reflectionPropertyHasType)},
		"hasdefaultvalue":   {Name: "hasDefaultValue", Method: phpobj.NativeMethod(reflectionPropertyHasDefaultValue)},
		"getdefaultvalue":   {Name: "getDefaultValue", Method: phpobj.NativeMethod(reflectionPropertyGetDefaultValue)},
		"getmodifiers":      {Name: "getModifiers", Method: phpobj.NativeMethod(reflectionPropertyGetModifiers)},
		"ispromoted":        {Name: "isPromoted", Method: phpobj.NativeMethod(reflectionPropertyIsPromoted)},
		"__tostring":        {Name: "__toString", Method: phpobj.NativeMethod(reflectionPropertyToString)},
		"setaccessible":     {Name: "setAccessible", Method: phpobj.NativeMethod(reflectionPropertySetAccessible)},
		"isfinal":           {Name: "isFinal", Method: phpobj.NativeMethod(reflectionPropertyIsFinal)},
		"isdynamic":         {Name: "isDynamic", Method: phpobj.NativeMethod(reflectionPropertyIsDynamic)},
		"isinitialized":     {Name: "isInitialized", Method: phpobj.NativeMethod(reflectionPropertyIsInitialized)},
		"getrawvalue":       {Name: "getRawValue", Method: phpobj.NativeMethod(reflectionPropertyGetRawValue)},
		"setrawvalue":       {Name: "setRawValue", Method: phpobj.NativeMethod(reflectionPropertySetRawValue)},
		"getmangledname":    {Name: "getMangledName", Method: phpobj.NativeMethod(reflectionPropertyGetMangledName)},
		"isprivateset":      {Name: "isPrivateSet", Method: phpobj.NativeMethod(reflectionPropertyIsPrivateSet)},
		"isprotectedset":    {Name: "isProtectedSet", Method: phpobj.NativeMethod(reflectionPropertyIsProtectedSet)},
		"ispublicset":       {Name: "isPublicSet", Method: phpobj.NativeMethod(reflectionPropertyIsPublicSet)},
		"getsettabletype":   {Name: "getSettableType", Method: phpobj.NativeMethod(reflectionPropertyGetSettableType)},
		"gethook":           {Name: "getHook", Method: phpobj.NativeMethod(reflectionPropertyGetHook)},
		"gethooks":          {Name: "getHooks", Method: phpobj.NativeMethod(reflectionPropertyGetHooks)},
		"hashook":           {Name: "hasHook", Method: phpobj.NativeMethod(reflectionPropertyHasHook)},
		"hashooks":          {Name: "hasHooks", Method: phpobj.NativeMethod(reflectionPropertyHasHooks)},
		"isvirtual":         {Name: "isVirtual", Method: phpobj.NativeMethod(reflectionPropertyIsVirtual)},
		"isabstract":        {Name: "isAbstract", Method: phpobj.NativeMethod(reflectionPropertyIsAbstract)},
	}
}

// reflectionPropertyGetDocComment returns the doc comment for a property.
// Doc comments are not preserved during compilation, so this always returns false.
func reflectionPropertyGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFalse.ZVal(), nil
}

func reflectionPropertyConstructFull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::__construct() expects exactly 2 arguments")
	}

	var class phpv.ZClass
	var obj phpv.ZObject
	var err error

	if args[0].GetType() == phpv.ZtObject {
		obj = args[0].AsObject(ctx)
		class = obj.GetClass()
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
		// Check for dynamic properties on the object instance
		if obj != nil {
			if zobj, ok := obj.(*phpobj.ZObject); ok {
				if v := zobj.HashTable().GetString(propName); v != nil {
					// Dynamic property found - create a synthetic prop entry
					prop = &phpv.ZClassProp{
						VarName:   propName,
						Modifiers: phpv.ZAttrPublic,
					}
					found = true
				}
			}
		}
		if !found {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", class.GetName(), propName))
		}
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

	// For static properties - getValue() with no args or getValue(null) both work
	if data.prop.Modifiers.IsStatic() {
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		v := staticProps.GetString(data.prop.VarName)
		if v != nil {
			return v, nil
		}
		// Check default value
		if data.prop.Default != nil {
			if cd, ok := data.prop.Default.(*phpv.CompileDelayed); ok {
				resolved, err := cd.Run(ctx)
				if err == nil {
					return resolved, nil
				}
			}
			return data.prop.Default.ZVal(), nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	// For instance properties, need an object argument
	if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionProperty::getValue(): argument must be an object for non-static properties")
	}

	obj := args[0].Value().(*phpobj.ZObject)
	if obj == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "ReflectionProperty::getValue(): argument must be an object for non-static properties")
	}
	// ReflectionProperty::getValue() bypasses visibility (PHP 8.1+).
	// Read directly using GetPropValue which handles private name mangling.
	v := obj.GetPropValue(data.prop)
	if v == nil {
		// Property not initialized - check for typed property error or return default
		if data.prop.TypeHint != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Typed property %s::$%s must not be accessed before initialization",
					data.class.Name, data.prop.VarName))
		}
		return phpv.ZNULL.ZVal(), nil
	}
	return v, nil
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
		// For static properties: setValue($value) or setValue($obj_or_null, $value)
		// When called with 1 arg, that arg is the value (deprecated since 8.5)
		// When called with 2 args, the second arg is the value (first is ignored)
		val := args[0]
		if len(args) >= 2 {
			val = args[1]
		} else {
			_ = ctx.Deprecated("Calling ReflectionProperty::setValue() with a single argument is deprecated", logopt.NoFuncName(true))
		}
		return nil, staticProps.SetString(data.prop.VarName, val)
	}

	// For instance properties
	if len(args) < 2 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setValue() expects an object and a value for non-static properties")
	}

	obj := args[0].Value().(*phpobj.ZObject)
	if obj == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setValue() expects an object and a value for non-static properties")
	}

	// ReflectionProperty::setValue() bypasses visibility (PHP 8.1+).
	// Enforce typed property type checking before setting.
	value := args[1]
	if data.prop.TypeHint != nil {
		if coerced, err := obj.EnforcePropertyType(ctx, data.prop.VarName, data.prop, value); err != nil {
			return nil, err
		} else if coerced != nil {
			value = coerced
		}
	}

	// Set directly, bypassing visibility checks
	return nil, obj.SetPropValueDirect(data.prop, value)
}

func reflectionPropertyGetDeclaringClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionClassObject(ctx, data.class)
}

func reflectionPropertyIsPromoted(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// A property is "promoted" if it has a Promotion modifier set
	// (constructor promotion like public function __construct(public string $name) {})
	// We check if any constructor args have Promotion set for this property name
	if data.class != nil {
		if m, ok := data.class.GetMethod("__construct"); ok {
			if fga, ok2 := m.Method.(phpv.FuncGetArgs); ok2 {
				for _, arg := range fga.GetArgs() {
					if arg.VarName == data.prop.VarName && arg.Promotion != 0 {
						return phpv.ZBool(true).ZVal(), nil
					}
				}
			}
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionPropertyToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZString("Property [ ]").ZVal(), nil
	}

	var sb strings.Builder
	sb.WriteString("Property [ ")

	access := data.prop.Modifiers.Access()
	if access == phpv.ZAttrProtected {
		sb.WriteString("protected")
	} else if access == phpv.ZAttrPrivate {
		sb.WriteString("private")
	} else {
		sb.WriteString("public")
	}
	// Asymmetric set visibility
	if data.prop.SetModifiers != 0 {
		setVis := "public"
		if data.prop.SetModifiers.IsProtected() {
			setVis = "protected"
		} else if data.prop.SetModifiers.IsPrivate() {
			setVis = "private"
		}
		sb.WriteString(" " + setVis + "(set)")
	} else if data.prop.Modifiers.IsReadonly() && !data.prop.Modifiers.IsPrivate() {
		// PHP 8.4: readonly implies protected(set)
		sb.WriteString(" protected(set)")
	}
	if data.prop.Modifiers.IsReadonly() {
		sb.WriteString(" readonly")
	}
	if data.prop.Modifiers.IsStatic() {
		sb.WriteString(" static")
	}
	if data.prop.TypeHint != nil {
		sb.WriteString(" " + data.prop.TypeHint.String())
	}
	sb.WriteString(fmt.Sprintf(" $%s", data.prop.VarName))
	sb.WriteString(" ]\n")

	return phpv.ZString(sb.String()).ZVal(), nil
}

func reflectionPropertySetAccessible(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// setAccessible has no effect since PHP 8.1, deprecated since 8.5
	_ = ctx.Deprecated("Method ReflectionProperty::setAccessible() is deprecated since 8.5, as it has no effect since PHP 8.1", logopt.NoFuncName(true))
	return phpv.ZNULL.ZVal(), nil
}

func reflectionPropertyIsFinal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.Has(phpv.ZAttrFinal)).ZVal(), nil
}

func reflectionPropertyIsDynamic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// All properties we reflect from ZClassProp are declared (not dynamic)
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionPropertyIsInitialized(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// For static properties
	if data.prop.Modifiers.IsStatic() {
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		if staticProps != nil {
			v := staticProps.GetString(data.prop.VarName)
			if v != nil {
				return phpv.ZBool(true).ZVal(), nil
			}
		}
		return phpv.ZBool(data.prop.Default != nil).ZVal(), nil
	}

	// For instance properties, need an object argument
	if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
		// No object provided - just check if there's a default
		return phpv.ZBool(data.prop.Default != nil).ZVal(), nil
	}

	obj := args[0].AsObject(ctx)
	v, err := obj.ObjectGet(ctx, data.prop.VarName)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(v != nil).ZVal(), nil
}

func reflectionPropertyGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.prop.Attributes, phpobj.AttributeTARGET_PROPERTY, name, flags)
}

// getRawValue bypasses hooks and reads the backing value directly
func reflectionPropertyGetRawValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	if data.prop.Modifiers.IsStatic() {
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		v := staticProps.GetString(data.prop.VarName)
		if v != nil {
			return v, nil
		}
		if data.prop.Default != nil {
			if cd, ok := data.prop.Default.(*phpv.CompileDelayed); ok {
				resolved, err := cd.Run(ctx)
				if err == nil {
					return resolved, nil
				}
			}
			return data.prop.Default.ZVal(), nil
		}
		return phpv.ZNULL.ZVal(), nil
	}

	if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::getRawValue(): argument must be an object for non-static properties")
	}

	obj := args[0].AsObject(ctx)
	// getRawValue bypasses hooks - read directly from hash table
	if data.prop.HasHooks && !data.prop.IsBacked {
		// Virtual property - cannot read raw value
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Must not write to virtual property %s::$%s", data.class.GetName(), data.prop.VarName))
	}
	zobj, ok := obj.(*phpobj.ZObject)
	if ok {
		v := zobj.GetPropValue(data.prop)
		if v != nil {
			return v, nil
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// setRawValue bypasses hooks and writes the backing value directly
func reflectionPropertySetRawValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	if data.prop.Modifiers.IsStatic() {
		if len(args) < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setRawValue() expects at least 1 argument for static properties")
		}
		staticProps, err := data.class.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		val := args[0]
		if len(args) >= 2 {
			val = args[1]
		}
		return nil, staticProps.SetString(data.prop.VarName, val)
	}

	if len(args) < 2 || args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionProperty::setRawValue() expects an object and a value for non-static properties")
	}

	obj := args[0].AsObject(ctx)
	if data.prop.HasHooks && !data.prop.IsBacked {
		// Virtual property - cannot write raw value
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Must not write to virtual property %s::$%s", data.class.GetName(), data.prop.VarName))
	}
	// setRawValue bypasses hooks - write directly to hash table
	zobj, ok := obj.(*phpobj.ZObject)
	if ok {
		return nil, zobj.HashTable().SetString(data.prop.VarName, args[1])
	}
	return nil, obj.ObjectSet(ctx, data.prop.VarName, args[1])
}

// getMangledName returns the internal mangled name of the property.
// Public properties: same as name. Protected: \0*\0name. Private: \0ClassName\0name.
func reflectionPropertyGetMangledName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	if data.prop.Modifiers.IsPrivate() {
		return phpv.ZString("\x00" + string(data.class.GetName()) + "\x00" + string(data.prop.VarName)).ZVal(), nil
	}
	if data.prop.Modifiers.IsProtected() {
		return phpv.ZString("\x00*\x00" + string(data.prop.VarName)).ZVal(), nil
	}
	return data.prop.VarName.ZVal(), nil
}

// Asymmetric visibility methods (PHP 8.4)
func reflectionPropertyIsPrivateSet(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.prop.SetModifiers != 0 {
		return phpv.ZBool(data.prop.SetModifiers.IsPrivate()).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsPrivate()).ZVal(), nil
}

func reflectionPropertyIsProtectedSet(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.prop.SetModifiers != 0 {
		return phpv.ZBool(data.prop.SetModifiers.IsProtected()).ZVal(), nil
	}
	// readonly implies protected(set)
	if data.prop.Modifiers.IsReadonly() && !data.prop.Modifiers.IsPrivate() {
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsProtected()).ZVal(), nil
}

func reflectionPropertyIsPublicSet(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.prop.SetModifiers != 0 {
		access := data.prop.SetModifiers.Access()
		return phpv.ZBool(access == phpv.ZAttrPublic || access == 0).ZVal(), nil
	}
	access := data.prop.Modifiers.Access()
	return phpv.ZBool(access == phpv.ZAttrPublic || access == 0).ZVal(), nil
}

// getSettableType returns the settable type for the property (for asymmetric visibility)
func reflectionPropertyGetSettableType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil || data.prop.TypeHint == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// For now, the settable type is the same as the property type
	return createReflectionTypeObject(ctx, data.prop.TypeHint)
}

// resolveHookType extracts the hook type string ("get" or "set") from a PropertyHookType enum or string argument.
func resolveHookType(ctx phpv.Context, arg *phpv.ZVal) string {
	if arg.GetType() == phpv.ZtObject {
		obj := arg.AsObject(ctx)
		if obj != nil {
			// PropertyHookType enum - get the backing value
			if backingVal := obj.HashTable().GetString("value"); backingVal != nil {
				return string(backingVal.AsString(ctx))
			}
			// Fallback to name
			if nameVal := obj.HashTable().GetString("name"); nameVal != nil {
				name := string(nameVal.AsString(ctx))
				switch name {
				case "Get":
					return "get"
				case "Set":
					return "set"
				}
			}
		}
	}
	return string(arg.AsString(ctx))
}

// createHookReflectionMethod creates a ReflectionMethod object for a property hook.
func createHookReflectionMethod(ctx phpv.Context, data *reflectionPropertyData, hookType string) (*phpv.ZVal, error) {
	var hook phpv.Runnable
	var hookName phpv.ZString

	switch hookType {
	case "get":
		hook = data.prop.GetHook
		hookName = phpv.ZString(fmt.Sprintf("$%s::get", data.prop.VarName))
	case "set":
		hook = data.prop.SetHook
		hookName = phpv.ZString(fmt.Sprintf("$%s::set", data.prop.VarName))
	}
	if hook == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Create a ZClassMethod that wraps the hook
	var method *phpv.ZClassMethod
	if hookType == "get" {
		method = &phpv.ZClassMethod{
			Name:  hookName,
			Class: data.class,
			Method: &phpv.HookCallable{
				Hook:     hook,
				HookName: string(data.class.GetName()) + "::$" + string(data.prop.VarName) + "::get",
			},
		}
	} else {
		paramName := data.prop.SetParam
		if paramName == "" {
			paramName = "value"
		}
		method = &phpv.ZClassMethod{
			Name:  hookName,
			Class: data.class,
			Method: &phpv.HookCallable{
				Hook:     hook,
				HookName: string(data.class.GetName()) + "::$" + string(data.prop.VarName) + "::set",
				Params: []*phpv.FuncArg{
					{VarName: paramName},
				},
			},
		}
	}

	// Create a ReflectionMethod object
	obj, err := phpobj.CreateZObject(ctx, ReflectionMethod)
	if err != nil {
		return nil, err
	}
	obj.HashTable().SetString("name", hookName.ZVal())
	obj.HashTable().SetString("class", data.class.GetName().ZVal())
	methodData := &reflectionMethodData{
		method: method,
		class:  data.class,
	}
	obj.SetOpaque(ReflectionMethod, methodData)
	return obj.ZVal(), nil
}

// getHook returns a ReflectionMethod for the specified hook type (PropertyHookType::Get or PropertyHookType::Set)
func reflectionPropertyGetHook(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionProperty::getHook() expects exactly 1 argument, 0 given")
	}
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	hookType := resolveHookType(ctx, args[0])
	switch hookType {
	case "get":
		if data.prop.GetHook != nil || data.prop.GetIsAbstract {
			return createHookReflectionMethod(ctx, data, "get")
		}
	case "set":
		if data.prop.SetHook != nil || data.prop.SetIsAbstract {
			return createHookReflectionMethod(ctx, data, "set")
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// getHooks returns an array of PropertyHookType => ReflectionMethod for all hooks
func reflectionPropertyGetHooks(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	if data.prop.GetHook != nil || data.prop.GetIsAbstract {
		rm, err := createHookReflectionMethod(ctx, data, "get")
		if err != nil {
			return nil, err
		}
		if !rm.IsNull() {
			arr.OffsetSet(ctx, phpv.ZString("get").ZVal(), rm)
		}
	}
	if data.prop.SetHook != nil || data.prop.SetIsAbstract {
		rm, err := createHookReflectionMethod(ctx, data, "set")
		if err != nil {
			return nil, err
		}
		if !rm.IsNull() {
			arr.OffsetSet(ctx, phpv.ZString("set").ZVal(), rm)
		}
	}
	return arr.ZVal(), nil
}

// hasHook checks if the property has a specific hook type
func reflectionPropertyHasHook(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionProperty::hasHook() expects exactly 1 argument, 0 given")
	}
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	hookType := resolveHookType(ctx, args[0])
	switch hookType {
	case "get":
		return phpv.ZBool(data.prop.GetHook != nil || data.prop.GetIsAbstract || data.prop.HasGetDeclared).ZVal(), nil
	case "set":
		return phpv.ZBool(data.prop.SetHook != nil || data.prop.SetIsAbstract || data.prop.HasSetDeclared).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

// hasHooks checks if the property has any hooks
func reflectionPropertyHasHooks(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.HasHooks).ZVal(), nil
}

// isVirtual checks if the property is virtual (has hooks but no backing store)
func reflectionPropertyIsVirtual(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// A property is virtual if it has hooks and no backing store
	if !data.prop.HasHooks {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(!data.prop.IsBacked).ZVal(), nil
}

// isAbstract checks if the property is abstract
func reflectionPropertyIsAbstract(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.Has(phpv.ZAttrAbstract)).ZVal(), nil
}

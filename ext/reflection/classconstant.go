package reflection

import (
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// reflectionClassConstantData is stored as opaque data on ReflectionClassConstant objects
type reflectionClassConstantData struct {
	constName phpv.ZString
	constVal  *phpv.ZClassConst
	class     *phpobj.ZClass
}

// ReflectionClassConstant modifier constants (same as ReflectionMethod)
const (
	ReflectionClassConstantIS_PUBLIC    int64 = 1
	ReflectionClassConstantIS_PROTECTED int64 = 2
	ReflectionClassConstantIS_PRIVATE   int64 = 4
	ReflectionClassConstantIS_FINAL     int64 = 32
)

func initReflectionClassConstant() {
	ReflectionClassConstant.Const = map[phpv.ZString]*phpv.ZClassConst{
		"IS_PUBLIC":    {Value: phpv.ZInt(ReflectionClassConstantIS_PUBLIC)},
		"IS_PROTECTED": {Value: phpv.ZInt(ReflectionClassConstantIS_PROTECTED)},
		"IS_PRIVATE":   {Value: phpv.ZInt(ReflectionClassConstantIS_PRIVATE)},
		"IS_FINAL":     {Value: phpv.ZInt(ReflectionClassConstantIS_FINAL)},
	}
	ReflectionClassConstant.Props = []*phpv.ZClassProp{
		{VarName: "name", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic, TypeHint: phpv.ParseTypeHint("string")},
		{VarName: "class", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
	}
	ReflectionClassConstant.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":       {Name: "__construct", Method: phpobj.NativeMethod(reflectionClassConstantConstruct)},
		"getname":           {Name: "getName", Method: phpobj.NativeMethod(reflectionClassConstantGetName)},
		"getvalue":          {Name: "getValue", Method: phpobj.NativeMethod(reflectionClassConstantGetValue)},
		"getdeclaringclass": {Name: "getDeclaringClass", Method: phpobj.NativeMethod(reflectionClassConstantGetDeclaringClass)},
		"getmodifiers":      {Name: "getModifiers", Method: phpobj.NativeMethod(reflectionClassConstantGetModifiers)},
		"ispublic":          {Name: "isPublic", Method: phpobj.NativeMethod(reflectionClassConstantIsPublic)},
		"isprotected":       {Name: "isProtected", Method: phpobj.NativeMethod(reflectionClassConstantIsProtected)},
		"isprivate":         {Name: "isPrivate", Method: phpobj.NativeMethod(reflectionClassConstantIsPrivate)},
		"isfinal":           {Name: "isFinal", Method: phpobj.NativeMethod(reflectionClassConstantIsFinal)},
		"isenumcase":        {Name: "isEnumCase", Method: phpobj.NativeMethod(reflectionClassConstantIsEnumCase)},
		"isdeprecated":      {Name: "isDeprecated", Method: phpobj.NativeMethod(reflectionClassConstantIsDeprecated)},
		"getattributes":     {Name: "getAttributes", Method: phpobj.NativeMethod(reflectionClassConstantGetAttributes)},
		"__tostring":        {Name: "__toString", Method: phpobj.NativeMethod(reflectionClassConstantToString)},
		"getdoccomment":     {Name: "getDocComment", Method: phpobj.NativeMethod(reflectionClassConstantGetDocComment)},
		"hastype":           {Name: "hasType", Method: phpobj.NativeMethod(reflectionClassConstantHasType)},
		"gettype":           {Name: "getType", Method: phpobj.NativeMethod(reflectionClassConstantGetType)},
	}
}

func reflectionClassConstantConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClassConstant::__construct() expects exactly 2 arguments")
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

	constName := args[1].AsString(ctx)

	zc, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" does not have a constant \"%s\"", class.GetName(), constName))
	}

	constVal, declaringClass := lookupClassConstWithDeclaringClass(zc, constName)
	if constVal == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Constant %s::%s does not exist", class.GetName(), constName))
	}

	data := &reflectionClassConstantData{
		constName: constName,
		constVal:  constVal,
		class:     declaringClass,
	}
	o.HashTable().SetString("name", constName.ZVal())
	o.HashTable().SetString("class", declaringClass.GetName().ZVal())
	o.SetOpaque(ReflectionClassConstant, data)
	return nil, nil
}

// lookupClassConst looks up a constant in the class and its parents.
func lookupClassConst(zc *phpobj.ZClass, name phpv.ZString) (*phpv.ZClassConst, bool) {
	v, _ := lookupClassConstWithDeclaringClass(zc, name)
	return v, v != nil
}

// lookupClassConstWithDeclaringClass looks up a constant in the class and its parents/interfaces,
// returning the constant value and the class that originally declared it.
func lookupClassConstWithDeclaringClass(zc *phpobj.ZClass, name phpv.ZString) (*phpv.ZClassConst, *phpobj.ZClass) {
	for cur := zc; cur != nil; {
		if cur.Const != nil {
			if v, ok := cur.Const[name]; ok {
				// If DeclaringClass is set, the constant was inherited from that class
				if v.DeclaringClass != nil {
					if dc, ok2 := v.DeclaringClass.(*phpobj.ZClass); ok2 {
						return v, dc
					}
				}
				return v, cur
			}
		}
		// Check implemented interfaces (for classes that inherit constants from interfaces)
		for _, intf := range cur.Implementations {
			if intf.Const != nil {
				if v, ok := intf.Const[name]; ok {
					return v, intf
				}
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
	return nil, nil
}

func getClassConstData(o *phpobj.ZObject) *reflectionClassConstantData {
	v := o.GetOpaque(ReflectionClassConstant)
	if v == nil {
		return nil
	}
	return v.(*reflectionClassConstantData)
}

func reflectionClassConstantGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if !o.HashTable().HasString("name") {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			"Typed property ReflectionClassConstant::$name must not be accessed before initialization")
	}
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.constName.ZVal(), nil
}

func reflectionClassConstantGetValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if data.constVal.Value == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// Value might be a CompileDelayed, resolve it
	val := data.constVal.Value
	var resolved *phpv.ZVal
	if cd, ok := val.(*phpv.CompileDelayed); ok {
		var err error
		resolved, err = cd.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		resolved = val.ZVal()
	}
	// Validate type if there's a type hint
	if data.constVal.TypeHint != nil && resolved != nil && !data.constVal.TypeHint.CheckStrict(ctx, resolved) {
		typeName := classConstValueTypeName(ctx, resolved)
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
				typeName, data.class.GetName(), data.constName, data.constVal.TypeHint.String()))
	}
	return resolved, nil
}

func reflectionClassConstantGetDeclaringClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionClassObject(ctx, data.class)
}

func reflectionClassConstantGetModifiers(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	var modifiers int64
	access := data.constVal.Modifiers.Access()
	switch access {
	case phpv.ZAttrPublic:
		modifiers |= ReflectionClassConstantIS_PUBLIC
	case phpv.ZAttrProtected:
		modifiers |= ReflectionClassConstantIS_PROTECTED
	case phpv.ZAttrPrivate:
		modifiers |= ReflectionClassConstantIS_PRIVATE
	default:
		// No access modifier set = public
		modifiers |= ReflectionClassConstantIS_PUBLIC
	}
	if data.constVal.Modifiers.Has(phpv.ZAttrFinal) {
		modifiers |= ReflectionClassConstantIS_FINAL
	}
	return phpv.ZInt(modifiers).ZVal(), nil
}

func reflectionClassConstantIsPublic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	access := data.constVal.Modifiers.Access()
	isPublic := access == phpv.ZAttrPublic || access == 0
	return phpv.ZBool(isPublic).ZVal(), nil
}

func reflectionClassConstantIsProtected(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.constVal.Modifiers.IsProtected()).ZVal(), nil
}

func reflectionClassConstantIsPrivate(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.constVal.Modifiers.IsPrivate()).ZVal(), nil
}

func reflectionClassConstantIsFinal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.constVal.Modifiers.Has(phpv.ZAttrFinal)).ZVal(), nil
}

func reflectionClassConstantIsEnumCase(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// A constant is an enum case if its declaring class is an enum
	// and the constant name appears in the EnumCases list
	if !data.class.GetType().Has(phpv.ZClassTypeEnum) {
		return phpv.ZBool(false).ZVal(), nil
	}

	for _, caseName := range data.class.EnumCases {
		if caseName == data.constName {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionClassConstantGetAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	name, flags := getAttributesArgs(ctx, args)
	return filterAttributes(ctx, data.constVal.Attributes, phpobj.AttributeTARGET_CLASS_CONSTANT, name, flags, data.class)
}

func reflectionClassConstantToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if !o.HashTable().HasString("name") {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			"Typed property ReflectionClassConstant::$name must not be accessed before initialization")
	}
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZString("Constant [ ]").ZVal(), nil
	}

	var accessStr string
	access := data.constVal.Modifiers.Access()
	switch access {
	case phpv.ZAttrProtected:
		accessStr = "protected"
	case phpv.ZAttrPrivate:
		accessStr = "private"
	default:
		accessStr = "public"
	}

	// final modifier comes before access modifier
	var modStr string
	if data.constVal.Modifiers.Has(phpv.ZAttrFinal) {
		modStr = "final " + accessStr
	} else {
		modStr = accessStr
	}

	// Resolve the constant value first
	var resolved *phpv.ZVal
	if data.constVal.Value != nil {
		if cd, ok := data.constVal.Value.(*phpv.CompileDelayed); ok {
			r, runErr := cd.Run(ctx)
			if runErr != nil {
				return nil, runErr
			}
			resolved = r
		} else {
			resolved = data.constVal.Value.ZVal()
		}
	}

	// Validate type if there's a type hint
	if data.constVal.TypeHint != nil && resolved != nil && !data.constVal.TypeHint.CheckStrict(ctx, resolved) {
		typeName := classConstValueTypeName(ctx, resolved)
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
				typeName, data.class.GetName(), data.constName, data.constVal.TypeHint.String()))
	}

	var typeStr string
	if data.constVal.TypeHint != nil {
		typeStr = data.constVal.TypeHint.String()
	} else if resolved != nil {
		// Use the type of the actual value
		switch resolved.GetType() {
		case phpv.ZtInt:
			typeStr = "int"
		case phpv.ZtFloat:
			typeStr = "float"
		case phpv.ZtString:
			typeStr = "string"
		case phpv.ZtBool:
			typeStr = "bool"
		case phpv.ZtNull:
			typeStr = "null"
		case phpv.ZtArray:
			typeStr = "array"
		default:
			typeStr = "mixed"
		}
	} else {
		typeStr = "mixed"
	}

	valStr := string(data.constName) // fallback
	if resolved != nil {
		valStr = formatConstantValue(ctx, resolved)
	}

	// Include doc comment if present
	docComment := data.constVal.DocComment
	var result string
	if docComment != "" {
		result = fmt.Sprintf("%s\nConstant [ %s %s %s ] { %s }\n", docComment, modStr, typeStr, data.constName, valStr)
	} else {
		result = fmt.Sprintf("Constant [ %s %s %s ] { %s }\n", modStr, typeStr, data.constName, valStr)
	}
	return phpv.ZString(result).ZVal(), nil
}

func reflectionClassConstantHasType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil || data.constVal.TypeHint == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionClassConstantGetType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil || data.constVal.TypeHint == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionTypeObject(ctx, data.constVal.TypeHint)
}

// createReflectionClassConstantObject creates a ReflectionClassConstant object
// for the given class and constant, without going through __construct.
func createReflectionClassConstantObject(ctx phpv.Context, class *phpobj.ZClass, name phpv.ZString, constVal *phpv.ZClassConst) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionClassConstant)
	if err != nil {
		return nil, err
	}
	data := &reflectionClassConstantData{
		constName: name,
		constVal:  constVal,
		class:     class,
	}
	obj.HashTable().SetString("name", name.ZVal())
	obj.HashTable().SetString("class", class.GetName().ZVal())
	obj.SetOpaque(ReflectionClassConstant, data)
	return obj.ZVal(), nil
}

// reflectionClassConstantGetDocComment returns the doc comment for a class constant.
func reflectionClassConstantGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data, ok := o.GetOpaque(ReflectionClassConstant).(*reflectionClassConstantData)
	if !ok || data == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if data.constVal.DocComment == "" {
		return phpv.ZFalse.ZVal(), nil
	}
	return data.constVal.DocComment.ZVal(), nil
}

// reflectionClassGetDocComment returns the doc comment for a class.
func reflectionClassGetDocComment(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if zc.DocComment == "" {
		return phpv.ZFalse.ZVal(), nil
	}
	return zc.DocComment.ZVal(), nil
}

// reflectionClassGetReflectionConstant returns a ReflectionClassConstant for the named constant.
// Returns false if the constant does not exist.
func reflectionClassGetReflectionConstant(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::getReflectionConstant() expects exactly 1 argument, 0 given")
	}

	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	name := args[0].AsString(ctx)
	constVal, found := lookupClassConst(zc, name)
	if !found {
		return phpv.ZBool(false).ZVal(), nil
	}

	return createReflectionClassConstantObject(ctx, zc, name, constVal)
}

// reflectionClassGetReflectionConstants returns an array of ReflectionClassConstant objects
// for all constants in the class.
func reflectionClassGetReflectionConstants(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Optional filter argument
	var filter int64 = -1 // -1 means no filter
	if len(args) > 0 && args[0].GetType() != phpv.ZtNull {
		filter = int64(args[0].AsInt(ctx))
	}

	arr := phpv.NewZArray()

	// Walk up the class hierarchy
	seen := make(map[string]bool)
	for cur := zc; cur != nil; {
		if cur.Const != nil {
			for _, name := range cur.ConstOrder {
				c := cur.Const[name]
				if c == nil {
					continue
				}
				key := strings.ToLower(string(name))
				if seen[key] {
					continue
				}

				// Skip private constants that are inherited from parent classes.
				// Private constants are only accessible on the class that declared them.
				if c.Modifiers.IsPrivate() && c.DeclaringClass != nil {
					seen[key] = true // mark as seen to prevent parent from re-adding
					continue
				}
				// When we walk to a parent class, skip its private constants (they're not visible on zc)
				if c.Modifiers.IsPrivate() && cur != zc {
					seen[key] = true
					continue
				}
				seen[key] = true

				// Apply filter if provided
				if filter != -1 && !classConstMatchesFilter(c, filter) {
					continue
				}

				// Use DeclaringClass if set (constant was inherited), otherwise use cur
				declaringClass := cur
				if c.DeclaringClass != nil {
					if dc, ok2 := c.DeclaringClass.(*phpobj.ZClass); ok2 {
						declaringClass = dc
					}
				}
				val, err := createReflectionClassConstantObject(ctx, declaringClass, name, c)
				if err != nil {
					return nil, err
				}
				arr.OffsetSet(ctx, nil, val)
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

	return arr.ZVal(), nil
}

// classConstMatchesFilter checks if a class constant matches a filter bitmask.
func classConstMatchesFilter(c *phpv.ZClassConst, filter int64) bool {
	access := c.Modifiers.Access()
	match := false

	if filter&ReflectionClassConstantIS_PUBLIC != 0 {
		if access == phpv.ZAttrPublic || access == 0 {
			match = true
		}
	}
	if filter&ReflectionClassConstantIS_PROTECTED != 0 {
		if access == phpv.ZAttrProtected {
			match = true
		}
	}
	if filter&ReflectionClassConstantIS_PRIVATE != 0 {
		if access == phpv.ZAttrPrivate {
			match = true
		}
	}

	if filter == 0 {
		return true
	}

	return match
}

// classConstValueTypeName returns a human-readable type name for a ZVal, used in type error messages.
func classConstValueTypeName(ctx phpv.Context, val *phpv.ZVal) string {
	if val == nil {
		return "null"
	}
	switch val.GetType() {
	case phpv.ZtObject:
		obj := val.AsObject(ctx)
		if obj != nil {
			return string(obj.GetClass().GetName())
		}
		return "object"
	case phpv.ZtArray:
		return "array"
	case phpv.ZtString:
		return "string"
	case phpv.ZtInt:
		return "int"
	case phpv.ZtFloat:
		return "float"
	case phpv.ZtBool:
		return "bool"
	case phpv.ZtNull:
		return "null"
	default:
		return "unknown"
	}
}

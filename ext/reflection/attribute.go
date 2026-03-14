package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionAttribute class
var ReflectionAttribute *phpobj.ZClass

// ReflectionAttribute flag constants
const (
	ReflectionAttributeIS_INSTANCEOF = 2
)

// reflectionAttributeData is stored as opaque data on ReflectionAttribute objects
type reflectionAttributeData struct {
	attr   *phpv.ZAttribute
	target int // AttributeTARGET_* constant
}

func initReflectionAttribute() {
	ReflectionAttribute = &phpobj.ZClass{
		Name: "ReflectionAttribute",
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_INSTANCEOF": {Value: phpv.ZInt(ReflectionAttributeIS_INSTANCEOF)},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"getname":      {Name: "getName", Method: phpobj.NativeMethod(reflectionAttributeGetName)},
			"getarguments": {Name: "getArguments", Method: phpobj.NativeMethod(reflectionAttributeGetArguments)},
			"gettarget":    {Name: "getTarget", Method: phpobj.NativeMethod(reflectionAttributeGetTarget)},
			"isrepeated":   {Name: "isRepeated", Method: phpobj.NativeMethod(reflectionAttributeIsRepeated)},
			"newinstance":  {Name: "newInstance", Method: phpobj.NativeMethod(reflectionAttributeNewInstance)},
			"__tostring":   {Name: "__toString", Method: phpobj.NativeMethod(reflectionAttributeToString)},
		},
	}
}

func getAttrData(o *phpobj.ZObject) *reflectionAttributeData {
	v := o.GetOpaque(ReflectionAttribute)
	if v == nil {
		return nil
	}
	return v.(*reflectionAttributeData)
}

func reflectionAttributeGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.attr.ClassName.ZVal(), nil
}

func reflectionAttributeGetArguments(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	arr := phpv.NewZArray()
	if data.attr.Args != nil {
		for _, arg := range data.attr.Args {
			arr.OffsetSet(ctx, nil, arg)
		}
	}
	return arr.ZVal(), nil
}

func reflectionAttributeGetTarget(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(data.target).ZVal(), nil
}

func reflectionAttributeIsRepeated(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// An attribute is "repeated" if it appears more than once on the same target.
	// This would require access to the full attribute list of the target.
	// For now, return false as a reasonable default.
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionAttributeNewInstance(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionAttribute::newInstance(): internal error")
	}

	// Look up the attribute class
	class, err := ctx.Global().GetClass(ctx, data.attr.ClassName, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attribute class \"%s\" not found", data.attr.ClassName))
	}

	// Create a new instance with the stored arguments
	var constructArgs []*phpv.ZVal
	if data.attr.Args != nil {
		constructArgs = data.attr.Args
	}

	obj, err := phpobj.NewZObject(ctx, class, constructArgs...)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func reflectionAttributeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.ZString("Attribute [ ]").ZVal(), nil
	}
	return phpv.ZString(fmt.Sprintf("Attribute [ %s ]", data.attr.ClassName)).ZVal(), nil
}

// createReflectionAttributeObject creates a ReflectionAttribute object for the given attribute.
func createReflectionAttributeObject(ctx phpv.Context, attr *phpv.ZAttribute, target int) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionAttribute)
	if err != nil {
		return nil, err
	}
	data := &reflectionAttributeData{
		attr:   attr,
		target: target,
	}
	obj.SetOpaque(ReflectionAttribute, data)
	return obj.ZVal(), nil
}

// filterAttributes returns ReflectionAttribute objects for matching attributes.
// name: optional class name filter (empty = all)
// flags: 0 or ReflectionAttribute::IS_INSTANCEOF
// attrs: the attributes to filter
// target: the AttributeTARGET_* constant for these attributes
func filterAttributes(ctx phpv.Context, attrs []*phpv.ZAttribute, target int, name phpv.ZString, flags int) (*phpv.ZVal, error) {
	arr := phpv.NewZArray()

	for _, attr := range attrs {
		if name != "" {
			if flags&ReflectionAttributeIS_INSTANCEOF != 0 {
				// Check if attribute class is an instance of the given name
				attrClass, err := ctx.Global().GetClass(ctx, attr.ClassName, false)
				if err != nil {
					continue
				}
				filterClass, err := ctx.Global().GetClass(ctx, name, false)
				if err != nil {
					continue
				}
				if !attrClass.InstanceOf(filterClass) {
					continue
				}
			} else {
				// Exact match
				if attr.ClassName.ToLower() != name.ToLower() {
					continue
				}
			}
		}

		val, err := createReflectionAttributeObject(ctx, attr, target)
		if err != nil {
			return nil, err
		}
		arr.OffsetSet(ctx, nil, val)
	}

	return arr.ZVal(), nil
}

// getAttributesArgs parses the common (name, flags) arguments for getAttributes()
func getAttributesArgs(ctx phpv.Context, args []*phpv.ZVal) (phpv.ZString, int) {
	var name phpv.ZString
	flags := 0

	if len(args) > 0 && args[0].GetType() != phpv.ZtNull {
		name = args[0].AsString(ctx)
	}
	if len(args) > 1 && args[1].GetType() != phpv.ZtNull {
		flags = int(args[1].AsInt(ctx))
	}

	return name, flags
}

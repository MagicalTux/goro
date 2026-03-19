package reflection

import (
	"fmt"
	"strings"

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
	attr     *phpv.ZAttribute
	target   int                // AttributeTARGET_* constant
	allAttrs []*phpv.ZAttribute // all attributes on the same target (for repeat checking)
}

func initReflectionAttribute() {
	ReflectionAttribute = &phpobj.ZClass{
		Name: "ReflectionAttribute",
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"IS_INSTANCEOF": {Value: phpv.ZInt(ReflectionAttributeIS_INSTANCEOF)},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":  {Name: "__construct", Method: phpobj.NativeMethod(reflectionAttributeConstruct)},
			"getname":      {Name: "getName", Method: phpobj.NativeMethod(reflectionAttributeGetName)},
			"getarguments": {Name: "getArguments", Method: phpobj.NativeMethod(reflectionAttributeGetArguments)},
			"gettarget":    {Name: "getTarget", Method: phpobj.NativeMethod(reflectionAttributeGetTarget)},
			"isrepeated":   {Name: "isRepeated", Method: phpobj.NativeMethod(reflectionAttributeIsRepeated)},
			"newinstance":  {Name: "newInstance", Method: phpobj.NativeMethod(reflectionAttributeNewInstance)},
			"__tostring":   {Name: "__toString", Method: phpobj.NativeMethod(reflectionAttributeToString)},
			"__debuginfo":  {Name: "__debugInfo", Method: phpobj.NativeMethod(reflectionAttributeDebugInfo)},
		},
	}
}

func reflectionAttributeConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, phpobj.ThrowError(ctx, ReflectionException, "Cannot directly instantiate ReflectionAttribute")
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

	// Resolve lazy argument expressions if needed
	resolveAttrArgs(ctx, data.attr)

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
	count := 0
	for _, a := range data.allAttrs {
		if a.ClassName.ToLower() == data.attr.ClassName.ToLower() {
			count++
		}
	}
	return phpv.ZBool(count > 1).ZVal(), nil
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

	// Validate that the class is actually an attribute class
	if !phpobj.IsAttributeClass(class) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attempting to use non-attribute class \"%s\" as attribute", data.attr.ClassName))
	}

	flags := phpobj.GetAttributeFlags(ctx, class)

	// Validate flags value is within valid range
	maxValid := int64(phpobj.AttributeTARGET_ALL | phpobj.AttributeIS_REPEATABLE)
	if flags < 0 || flags > maxValid {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid attribute flags specified")
	}

	// Validate target
	if int(flags)&data.target == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attribute \"%s\" cannot target %s (allowed targets: %s)",
				data.attr.ClassName,
				phpobj.TargetName(data.target),
				describeTargetsForReflection(int(flags))))
	}

	// Validate repeat
	if int(flags)&phpobj.AttributeIS_REPEATABLE == 0 {
		count := 0
		for _, a := range data.allAttrs {
			if a.ClassName.ToLower() == data.attr.ClassName.ToLower() {
				count++
			}
		}
		if count > 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Attribute \"%s\" must not be repeated", data.attr.ClassName))
		}
	}

	// Resolve lazy argument expressions if needed
	resolveAttrArgs(ctx, data.attr)

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

// describeTargetsForReflection returns a human-readable string of allowed targets.
func describeTargetsForReflection(flags int) string {
	var parts []string
	if flags&phpobj.AttributeTARGET_CLASS != 0 {
		parts = append(parts, "class")
	}
	if flags&phpobj.AttributeTARGET_FUNCTION != 0 {
		parts = append(parts, "function")
	}
	if flags&phpobj.AttributeTARGET_METHOD != 0 {
		parts = append(parts, "method")
	}
	if flags&phpobj.AttributeTARGET_PROPERTY != 0 {
		parts = append(parts, "property")
	}
	if flags&phpobj.AttributeTARGET_CLASS_CONSTANT != 0 {
		parts = append(parts, "class constant")
	}
	if flags&phpobj.AttributeTARGET_PARAMETER != 0 {
		parts = append(parts, "parameter")
	}
	if flags&phpobj.AttributeTARGET_CONSTANT != 0 {
		parts = append(parts, "constant")
	}
	if len(parts) == 0 {
		return "none"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}

func reflectionAttributeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.ZString("Attribute [ ]").ZVal(), nil
	}

	// Resolve lazy argument expressions if needed
	resolveAttrArgs(ctx, data.attr)

	if len(data.attr.Args) == 0 {
		return phpv.ZString(fmt.Sprintf("Attribute [ %s ]\n", data.attr.ClassName)).ZVal(), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Attribute [ %s ] {\n", data.attr.ClassName))
	sb.WriteString(fmt.Sprintf("  - Arguments [%d] {\n", len(data.attr.Args)))
	for i, arg := range data.attr.Args {
		sb.WriteString(fmt.Sprintf("    Argument #%d [ ", i))
		if arg != nil {
			sb.WriteString(arg.String())
		}
		sb.WriteString(" ]\n")
	}
	sb.WriteString("  }\n}\n")
	return phpv.ZString(sb.String()).ZVal(), nil
}

func reflectionAttributeDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getAttrData(o)
	if data == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZString("name"), data.attr.ClassName.ZVal())
	return arr.ZVal(), nil
}

// resolveAttrArgs evaluates any lazy argument expressions on the attribute.
// This is called at runtime when getArguments() or newInstance() is invoked.
func resolveAttrArgs(ctx phpv.Context, attr *phpv.ZAttribute) {
	if attr.ArgExprs == nil {
		return
	}
	for i, expr := range attr.ArgExprs {
		if expr != nil {
			val, err := expr.Run(ctx)
			if err == nil && val != nil {
				attr.Args[i] = val
				attr.ArgExprs[i] = nil // mark as resolved
			}
		}
	}
}

// createReflectionAttributeObject creates a ReflectionAttribute object for the given attribute.
func createReflectionAttributeObject(ctx phpv.Context, attr *phpv.ZAttribute, target int, allAttrs []*phpv.ZAttribute) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionAttribute)
	if err != nil {
		return nil, err
	}
	data := &reflectionAttributeData{
		attr:     attr,
		target:   target,
		allAttrs: allAttrs,
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

		val, err := createReflectionAttributeObject(ctx, attr, target, attrs)
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

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
	attr        *phpv.ZAttribute
	target      int                // AttributeTARGET_* constant
	allAttrs    []*phpv.ZAttribute // all attributes on the same target (for repeat checking)
	targetClass phpv.ZClass        // the class the attribute is declared on (for validator checks)
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
	if err := resolveAttrArgs(ctx, data.attr); err != nil {
		return nil, err
	}

	arr := phpv.NewZArray()
	if data.attr.Args != nil {
		for i, arg := range data.attr.Args {
			// Use named key if available
			if i < len(data.attr.ArgNames) && data.attr.ArgNames[i] != "" {
				arr.OffsetSet(ctx, data.attr.ArgNames[i], arg)
			} else {
				arr.OffsetSet(ctx, nil, arg)
			}
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

	// Validate the #[Attribute] attribute on the class by instantiating it.
	// This catches type errors like #[Attribute("foo")] where "foo" is not int,
	// and also resolves lazy arg expressions (like Foo::BAR).
	flags := int64(phpobj.AttributeTARGET_ALL) // default
	zc, _ := class.(*phpobj.ZClass)
	if zc != nil {
		for _, classAttr := range zc.Attributes {
			if classAttr.ClassName == "Attribute" || classAttr.ClassName == "\\Attribute" {
				// Resolve lazy args
				if err := resolveAttrArgs(ctx, classAttr); err != nil {
					return nil, err
				}
				if len(classAttr.Args) > 0 {
					// Validate by instantiating the Attribute class
					_, err := phpobj.NewZObject(ctx, phpobj.AttributeClass, classAttr.Args...)
					if err != nil {
						return nil, err
					}
					flags = int64(classAttr.Args[0].AsInt(ctx))
				}
				break
			}
		}
	}

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

	// Special validator checks for built-in attribute classes
	// These checks run at newInstance() time, especially for #[DelayedTargetValidation]
	if data.targetClass != nil {
		if err := validateBuiltinAttributeOnClass(ctx, data.attr.ClassName, data.targetClass); err != nil {
			return nil, err
		}
	}

	// Resolve lazy argument expressions if needed
	if err := resolveAttrArgs(ctx, data.attr); err != nil {
		return nil, err
	}

	// Create a new instance with the stored arguments
	var constructArgs []*phpv.ZVal
	if data.attr.Args != nil {
		constructArgs = make([]*phpv.ZVal, len(data.attr.Args))
		copy(constructArgs, data.attr.Args)
	}

	// Resolve named arguments to the correct positional parameters
	// based on the constructor's parameter list
	if data.attr.ArgNames != nil {
		var constructor phpv.Callable
		if zc != nil {
			if zc.Handlers() != nil && zc.Handlers().Constructor != nil {
				constructor = zc.Handlers().Constructor.Method
			} else if m, ok := zc.GetMethod("__construct"); ok {
				constructor = m.Method
			}
		}

		// Check for named args: if there's no constructor or no FuncGetArgs,
		// any named argument is unknown
		hasNamedArgs := false
		for _, name := range data.attr.ArgNames {
			if name != "" {
				hasNamedArgs = true
				break
			}
		}

		if hasNamedArgs && constructor == nil {
			// No constructor but named args used - any named arg is unknown
			for _, name := range data.attr.ArgNames {
				if name != "" {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Unknown named parameter $%s", name))
				}
			}
		}

		if constructor != nil {
			if fga, ok := constructor.(phpv.FuncGetArgs); ok {
				fargs := fga.GetArgs()
				// Build parameter name -> position map
				paramMap := make(map[phpv.ZString]int)
				for i, arg := range fargs {
					paramMap[phpv.ZString(arg.VarName)] = i
				}
				// Build new args array with named args placed at correct positions
				// First count positional args (those without names)
				positionalCount := 0
				for i, name := range data.attr.ArgNames {
					if name == "" && i < len(constructArgs) {
						positionalCount = i + 1
					}
				}
				// Determine max position needed
				maxPos := positionalCount - 1
				for i, name := range data.attr.ArgNames {
					if name != "" {
						if pos, ok := paramMap[name]; ok && pos > maxPos {
							maxPos = pos
						} else if !ok {
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								fmt.Sprintf("Unknown named parameter $%s", name))
						}
					} else if i > maxPos {
						maxPos = i
					}
				}
				// Build properly ordered args
				newArgs := make([]*phpv.ZVal, maxPos+1)
				for i := range newArgs {
					newArgs[i] = phpv.ZNULL.ZVal()
				}
				// Place positional args
				namedIdx := 0
				for i, name := range data.attr.ArgNames {
					if i >= len(constructArgs) {
						break
					}
					if name == "" {
						if namedIdx < len(newArgs) {
							newArgs[namedIdx] = constructArgs[i]
						}
						namedIdx++
					} else {
						if pos, ok := paramMap[name]; ok {
							newArgs[pos] = constructArgs[i]
						}
					}
				}
				// Also handle args without names list entries
				if len(data.attr.ArgNames) == 0 {
					newArgs = constructArgs
				}
				constructArgs = newArgs
			} else if hasNamedArgs {
				// Constructor exists but doesn't support named args introspection
				for _, name := range data.attr.ArgNames {
					if name != "" {
						return nil, phpobj.ThrowError(ctx, phpobj.Error,
							fmt.Sprintf("Unknown named parameter $%s", name))
					}
				}
			}
		}
	}

	// Use global context so constructor visibility check uses "global scope"
	// instead of "scope ReflectionAttribute"
	obj, err := phpobj.NewZObject(ctx.Global(), class, constructArgs...)
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
	if err := resolveAttrArgs(ctx, data.attr); err != nil {
		return nil, err
	}

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
func resolveAttrArgs(ctx phpv.Context, attr *phpv.ZAttribute) error {
	if attr.ArgExprs == nil {
		return nil
	}
	for i, expr := range attr.ArgExprs {
		if expr != nil {
			val, err := expr.Run(ctx)
			if err != nil {
				return err
			}
			if val != nil {
				attr.Args[i] = val
				attr.ArgExprs[i] = nil // mark as resolved
			}
		}
	}
	return nil
}

// createReflectionAttributeObject creates a ReflectionAttribute object for the given attribute.
func createReflectionAttributeObject(ctx phpv.Context, attr *phpv.ZAttribute, target int, allAttrs []*phpv.ZAttribute) (*phpv.ZVal, error) {
	return createReflectionAttributeObjectWithClass(ctx, attr, target, allAttrs, nil)
}

// createReflectionAttributeObjectWithClass creates a ReflectionAttribute object with target class info.
func createReflectionAttributeObjectWithClass(ctx phpv.Context, attr *phpv.ZAttribute, target int, allAttrs []*phpv.ZAttribute, targetClass phpv.ZClass) (*phpv.ZVal, error) {
	obj, err := phpobj.CreateZObject(ctx, ReflectionAttribute)
	if err != nil {
		return nil, err
	}
	data := &reflectionAttributeData{
		attr:        attr,
		target:      target,
		allAttrs:    allAttrs,
		targetClass: targetClass,
	}
	obj.SetOpaque(ReflectionAttribute, data)
	return obj.ZVal(), nil
}

// validateBuiltinAttributeOnClass performs validator checks for built-in attribute classes
// when newInstance() is called. These checks are deferred by #[DelayedTargetValidation].
func validateBuiltinAttributeOnClass(ctx phpv.Context, attrName phpv.ZString, targetClass phpv.ZClass) error {
	zc, ok := targetClass.(*phpobj.ZClass)
	if !ok {
		return nil
	}

	switch attrName {
	case "Attribute", "\\Attribute":
		if zc.Type.Has(phpv.ZClassTypeInterface) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Attribute] to interface %s", zc.GetName()))
		} else if zc.Type.Has(phpv.ZClassTypeTrait) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Attribute] to trait %s", zc.GetName()))
		} else if zc.Type.Has(phpv.ZClassTypeEnum) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Attribute] to enum %s", zc.GetName()))
		} else if zc.Attr.Has(phpv.ZClassExplicitAbstract) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Attribute] to abstract class %s", zc.GetName()))
		}
	case "AllowDynamicProperties", "\\AllowDynamicProperties":
		if zc.Type.Has(phpv.ZClassTypeInterface) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to interface %s", zc.GetName()))
		} else if zc.Type.Has(phpv.ZClassTypeTrait) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to trait %s", zc.GetName()))
		} else if zc.Type.Has(phpv.ZClassTypeEnum) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to enum %s", zc.GetName()))
		} else if zc.Attr.Has(phpv.ZClassReadonly) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\AllowDynamicProperties] to readonly class %s", zc.GetName()))
		}
	case "Deprecated", "\\Deprecated":
		// #[Deprecated] is valid on traits but NOT on classes/interfaces/enums
		if zc.Type.Has(phpv.ZClassTypeTrait) {
			// Traits are OK
		} else if zc.Type.Has(phpv.ZClassTypeInterface) {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Deprecated] to interface %s", zc.GetName()))
		} else if zc.GetType()&phpv.ZClassTypeEnum != 0 {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Deprecated] to enum %s", zc.GetName()))
		} else {
			// Regular class/abstract class
			kind := "class"
			if zc.Attr.Has(phpv.ZClassExplicitAbstract) {
				kind = "class" // PHP says "class" even for abstract
			}
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot apply #[\\Deprecated] to %s %s", kind, zc.GetName()))
		}
	case "NoDiscard", "\\NoDiscard":
		// #[NoDiscard] on classes is not valid (only functions/methods)
		// But since target validation already handles this, only check for special cases
		// NoDiscard target is function|method, so class target would already be caught
	}

	return nil
}

// filterAttributes returns ReflectionAttribute objects for matching attributes.
// name: optional class name filter (empty = all)
// flags: 0 or ReflectionAttribute::IS_INSTANCEOF
// attrs: the attributes to filter
// target: the AttributeTARGET_* constant for these attributes
func filterAttributes(ctx phpv.Context, attrs []*phpv.ZAttribute, target int, name phpv.ZString, flags int, targetClass ...phpv.ZClass) (*phpv.ZVal, error) {
	// Validate flags: only 0 and IS_INSTANCEOF (2) are valid
	if flags != 0 && flags != ReflectionAttributeIS_INSTANCEOF {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"ReflectionFunctionAbstract::getAttributes(): Argument #2 ($flags) must be a valid attribute filter flag")
	}

	// When IS_INSTANCEOF is set and a name is given, the filter class must exist
	if name != "" && flags&ReflectionAttributeIS_INSTANCEOF != 0 {
		_, err := ctx.Global().GetClass(ctx, name, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Class \"%s\" not found", name))
		}
	}

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

		var tc phpv.ZClass
		if len(targetClass) > 0 {
			tc = targetClass[0]
		}
		val, err := createReflectionAttributeObjectWithClass(ctx, attr, target, attrs, tc)
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

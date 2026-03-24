package reflection

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func reflectionClassToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return phpv.ZString(formatReflectionClass(ctx, zc)).ZVal(), nil
}

func reflectionClassHasConstant(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZBool(false).ZVal(), nil
	}
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := args[0].AsString(ctx)
	_, found := lookupClassConst(zc, name)
	return phpv.ZBool(found).ZVal(), nil
}

func reflectionClassGetConstant(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZBool(false).ZVal(), nil
	}
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := args[0].AsString(ctx)
	constVal, found := lookupClassConst(zc, name)
	if !found {
		_ = ctx.Deprecated("ReflectionClass::getConstant() for a non-existent constant is deprecated, use ReflectionClass::hasConstant() to check if the constant exists", logopt.NoFuncName(true))
		return phpv.ZBool(false).ZVal(), nil
	}
	if constVal.Value == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if cd, ok := constVal.Value.(*phpv.CompileDelayed); ok {
		resolved, err := cd.Run(ctx)
		if err != nil {
			return nil, err
		}
		return resolved, nil
	}
	return constVal.Value.ZVal(), nil
}

func reflectionClassGetDefaultProperties(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	seen := make(map[string]bool)
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			key := string(prop.VarName)
			if seen[key] {
				continue
			}
			if cur != zc && prop.Modifiers.IsPrivate() {
				continue
			}
			seen[key] = true
			val := prop.Default
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
			// Resolve CompileDelayed values
			if cd, ok := val.(*phpv.CompileDelayed); ok {
				resolved, err := cd.Run(ctx)
				if err != nil {
					continue
				}
				arr.OffsetSet(ctx, prop.VarName, resolved)
			} else {
				arr.OffsetSet(ctx, prop.VarName, val.ZVal())
			}
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		cur = parent.(*phpobj.ZClass)
	}
	return arr.ZVal(), nil
}

func reflectionClassGetStaticProperties(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	staticProps, err := zc.GetStaticProps(ctx)
	if err != nil {
		return nil, err
	}
	arr := phpv.NewZArray()
	if staticProps != nil {
		it := staticProps.NewIterator()
		for it.Valid(ctx) {
			k, _ := it.Key(ctx)
			v, _ := it.Current(ctx)
			if v != nil {
				arr.OffsetSet(ctx, k.Value(), v)
			}
			it.Next(ctx)
		}
	}
	return arr.ZVal(), nil
}

func reflectionClassGetStaticPropertyValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getStaticPropertyValue() expects at least 1 argument, 0 given")
	}
	if len(args) > 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::getStaticPropertyValue() expects at most 2 arguments, %d given", len(args)))
	}
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::getStaticPropertyValue(): Argument #1 ($name) must be of type string, array given")
	}
	if args[0].GetType() == phpv.ZtNull {
		_ = ctx.Deprecated("Passing null to parameter #1 ($name) of type string is deprecated")
	}
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	name := args[0].AsString(ctx)
	staticProps, err := zc.GetStaticProps(ctx)
	if err != nil {
		return nil, err
	}
	if staticProps != nil {
		v := staticProps.GetString(name)
		if v != nil && v.GetType() != phpv.ZtNull {
			return v, nil
		}
	}
	if len(args) > 1 {
		return args[1], nil
	}
	return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Property %s::$%s does not exist", zc.GetName(), name))
}

func reflectionClassSetStaticPropertyValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::setStaticPropertyValue() expects exactly 2 arguments, %d given", len(args)))
	}
	if len(args) > 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::setStaticPropertyValue() expects exactly 2 arguments, %d given", len(args)))
	}
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionClass::setStaticPropertyValue(): Argument #1 ($name) must be of type string, array given")
	}
	if args[0].GetType() == phpv.ZtNull {
		_ = ctx.Deprecated("Passing null to parameter #1 ($name) of type string is deprecated")
	}
	zc := getZClass(o)
	if zc == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}
	name := args[0].AsString(ctx)
	staticProps, err := zc.GetStaticProps(ctx)
	if err != nil {
		return nil, err
	}
	if staticProps != nil {
		// Check if the property exists as a static property first
		found := false
		for cur := zc; cur != nil; {
			for _, prop := range cur.Props {
				if prop.VarName == name && prop.Modifiers.IsStatic() {
					found = true
					break
				}
			}
			if found {
				break
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
		if found {
			return nil, staticProps.SetString(name, args[1])
		}
	}
	return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class %s does not have a property named %s", zc.GetName(), name))
}

func reflectionClassNewInstanceArgs(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}
	var constructArgs []*phpv.ZVal
	if len(args) > 0 {
		if args[0].GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionClass::newInstanceArgs(): Argument #1 ($args) must be of type array, %s given", args[0].GetType().String()))
		}
		arr := args[0].Value().(*phpv.ZArray)
		for _, v := range arr.Iterate(ctx) {
			constructArgs = append(constructArgs, v)
		}
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
		if !hasConstructor && len(constructArgs) > 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class %s does not have a constructor, so you cannot pass any constructor arguments", class.GetName()))
		}
	}

	obj, err := phpobj.NewZObject(ctx, class, constructArgs...)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func reflectionClassIsCloneable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if zc.Type == phpv.ZClassTypeInterface || zc.Type.Has(phpv.ZClassTypeTrait) {
		return phpv.ZBool(false).ZVal(), nil
	}
	if zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		return phpv.ZBool(false).ZVal(), nil
	}
	if m, ok := zc.GetMethod("__clone"); ok {
		if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
			return phpv.ZBool(false).ZVal(), nil
		}
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionClassIsAnonymous(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Attr.Has(phpv.ZClassAttr(phpv.ZClassAnon))).ZVal(), nil
}

func reflectionClassIsEnum(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Type.Has(phpv.ZClassTypeEnum)).ZVal(), nil
}

func reflectionClassIsTrait(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Type.Has(phpv.ZClassTypeTrait)).ZVal(), nil
}

func reflectionClassIsReadOnly(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.Attr.Has(phpv.ZClassReadonly)).ZVal(), nil
}

func reflectionClassIsIterable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Interfaces and traits are not iterable
	if zc.Type == phpv.ZClassTypeInterface || zc.Type.Has(phpv.ZClassTypeTrait) {
		return phpv.ZBool(false).ZVal(), nil
	}
	traversable, err := ctx.Global().GetClass(ctx, "Traversable", false)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	var class phpv.ZClass = zc
	return phpv.ZBool(class.InstanceOf(traversable)).ZVal(), nil
}

func reflectionClassIsInstance(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionClass::isInstance() expects exactly 1 argument, 0 given")
	}
	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if args[0].GetType() != phpv.ZtObject {
		return phpv.ZBool(false).ZVal(), nil
	}
	obj := args[0].AsObject(ctx)
	if obj == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(obj.GetClass().InstanceOf(class)).ZVal(), nil
}

func reflectionClassIsInternal(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.L == nil).ZVal(), nil
}

func reflectionClassIsUserDefined(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(zc.L != nil).ZVal(), nil
}

func reflectionClassGetFileName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil || zc.L == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(zc.L.Filename).ZVal(), nil
}

func reflectionClassGetStartLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil || zc.L == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(zc.L.Line).ZVal(), nil
}

func reflectionClassGetEndLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil || zc.L == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(zc.L.Line).ZVal(), nil
}

func reflectionClassGetModifiers(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	var modifiers int64
	if zc.Attr.Has(phpv.ZClassAttr(phpv.ZClassExplicitAbstract)) {
		modifiers |= 64 // IS_EXPLICIT_ABSTRACT
	}
	// Note: interfaces do NOT have IS_IMPLICIT_ABSTRACT in PHP 8.x
	if zc.Attr.Has(phpv.ZClassFinal) {
		modifiers |= 32 // IS_FINAL
	}
	if zc.Attr.Has(phpv.ZClassReadonly) {
		modifiers |= 65536 // IS_READONLY
	}
	return phpv.ZInt(modifiers).ZVal(), nil
}

func reflectionClassGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func reflectionClassGetExtensionName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionClassGetShortName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZString("").ZVal(), nil
	}
	name := string(class.GetName())
	if idx := strings.LastIndex(name, "\\"); idx >= 0 {
		return phpv.ZString(name[idx+1:]).ZVal(), nil
	}
	return phpv.ZString(name).ZVal(), nil
}

func reflectionClassGetNamespaceName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZString("").ZVal(), nil
	}
	name := string(class.GetName())
	if idx := strings.LastIndex(name, "\\"); idx >= 0 {
		return phpv.ZString(name[:idx]).ZVal(), nil
	}
	return phpv.ZString("").ZVal(), nil
}

func reflectionClassInNamespace(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := getClassData(o)
	if class == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(strings.Contains(string(class.GetName()), "\\")).ZVal(), nil
}

func reflectionClassGetInterfaces(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	seen := make(map[string]bool)
	var collectInterfaces func(c *phpobj.ZClass)
	collectInterfaces = func(c *phpobj.ZClass) {
		for _, impl := range c.Implementations {
			key := strings.ToLower(string(impl.GetName()))
			if seen[key] {
				continue
			}
			seen[key] = true
			rcVal, err := createReflectionClassObject(ctx, impl)
			if err == nil {
				arr.OffsetSet(ctx, impl.GetName(), rcVal)
			}
			collectInterfaces(impl)
		}
		parent := c.GetParent()
		if !phpv.IsNilClass(parent) {
			if pc, ok := parent.(*phpobj.ZClass); ok {
				collectInterfaces(pc)
			}
		}
	}
	collectInterfaces(zc)
	return arr.ZVal(), nil
}

func reflectionClassGetTraits(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	for _, traitUse := range zc.TraitUses {
		for _, traitName := range traitUse.TraitNames {
			traitClass, err := ctx.Global().GetClass(ctx, traitName, false)
			if err != nil {
				continue
			}
			val, err := createReflectionClassObject(ctx, traitClass)
			if err == nil {
				arr.OffsetSet(ctx, traitClass.GetName(), val)
			}
		}
	}
	return arr.ZVal(), nil
}

func reflectionClassGetTraitNames(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	for _, traitUse := range zc.TraitUses {
		for _, traitName := range traitUse.TraitNames {
			traitClass, err := ctx.Global().GetClass(ctx, traitName, false)
			if err != nil {
				arr.OffsetSet(ctx, nil, traitName.ZVal())
			} else {
				arr.OffsetSet(ctx, nil, traitClass.GetName().ZVal())
			}
		}
	}
	return arr.ZVal(), nil
}

func reflectionClassGetTraitAliases(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	arr := phpv.NewZArray()
	for _, traitUse := range zc.TraitUses {
		for _, alias := range traitUse.Aliases {
			if alias.NewName != "" {
				traitName := alias.TraitName
				if traitName == "" && len(traitUse.TraitNames) > 0 {
					traitName = traitUse.TraitNames[0]
				}
				// Key is the alias name, value is "TraitName::methodName"
				arr.OffsetSet(ctx, alias.NewName, phpv.ZString(string(traitName)+"::"+string(alias.MethodName)).ZVal())
			}
		}
	}
	return arr.ZVal(), nil
}

// formatReflectionClass generates a PHP-compatible string representation of a ReflectionClass.
func formatReflectionClass(ctx phpv.Context, zc *phpobj.ZClass) string {
	var sb strings.Builder

	kind := "Class"
	kindLower := "class"
	if zc.Type.Has(phpv.ZClassTypeInterface) {
		kind = "Interface"
		kindLower = "interface"
	} else if zc.Type.Has(phpv.ZClassTypeTrait) {
		kind = "Trait"
		kindLower = "trait"
	} else if zc.GetType()&phpv.ZClassTypeEnum != 0 {
		// Use GetType() instead of Type to avoid matching non-enum flags
		kind = "Enum"
		kindLower = "enum"
	}

	origin := "<user>"
	if zc.L == nil {
		origin = "<internal>"
	}

	iterateable := ""
	traversable, err := ctx.Global().GetClass(ctx, "Traversable", false)
	if err == nil {
		var class phpv.ZClass = zc
		if class.InstanceOf(traversable) {
			iterateable = " <iterateable>"
		}
	}

	modifiers := ""
	if zc.Attr.Has(phpv.ZClassAttr(phpv.ZClassExplicitAbstract)) {
		modifiers += " abstract"
	}
	if zc.Attr.Has(phpv.ZClassFinal) && kind != "Enum" {
		// Enums are implicitly final but don't show it in the format
		modifiers += " final"
	}
	if zc.Attr.Has(phpv.ZClassReadonly) {
		modifiers += " readonly"
	}

	sb.WriteString(fmt.Sprintf("%s [ %s%s%s %s %s",
		kind, origin, iterateable, modifiers, kindLower, string(zc.GetName())))

	if zc.Extends != nil {
		sb.WriteString(" extends " + string(zc.Extends.GetName()))
	}
	if len(zc.Implementations) > 0 {
		if zc.Type.Has(phpv.ZClassTypeInterface) {
			sb.WriteString(" extends ")
		} else {
			sb.WriteString(" implements ")
		}
		for i, impl := range zc.Implementations {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(string(impl.GetName()))
		}
	}
	sb.WriteString(" ] {\n")
	if zc.L != nil {
		sb.WriteString(fmt.Sprintf("  @@ %s %d-%d\n", zc.L.Filename, zc.L.Line, zc.L.Line))
	}
	sb.WriteString("\n")

	constCount := 0
	if zc.Const != nil {
		constCount = len(zc.Const)
	}
	sb.WriteString(fmt.Sprintf("  - Constants [%d] {\n", constCount))
	if zc.Const != nil {
		for _, name := range zc.ConstOrder {
			c := zc.Const[name]
			if c == nil {
				continue
			}
			modStr := "public"
			if c.Modifiers.IsProtected() {
				modStr = "protected"
			} else if c.Modifiers.IsPrivate() {
				modStr = "private"
			}
			typeStr := "mixed"
			if c.TypeHint != nil {
				typeStr = c.TypeHint.String()
			}
			valStr := string(name) // fallback
			if c.Value != nil {
				if cd, ok := c.Value.(*phpv.CompileDelayed); ok {
					resolved, err := cd.Run(ctx)
					if err == nil && resolved != nil {
						valStr = formatConstantValue(ctx, resolved)
					}
				} else {
					valStr = formatConstantValue(ctx, c.Value.ZVal())
				}
			}
			sb.WriteString(fmt.Sprintf("    Constant [ %s %s %s ] { %s }\n", modStr, typeStr, name, valStr))
		}
	}
	sb.WriteString("  }\n\n")

	staticCount := 0
	for _, prop := range zc.Props {
		if prop.Modifiers.IsStatic() {
			staticCount++
		}
	}
	sb.WriteString(fmt.Sprintf("  - Static properties [%d] {\n", staticCount))
	for _, prop := range zc.Props {
		if !prop.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatStaticProperty(ctx, prop))
	}
	sb.WriteString("  }\n\n")

	staticMethodCount := 0
	for _, m := range zc.Methods {
		if m.Modifiers.IsStatic() {
			staticMethodCount++
		}
	}
	sb.WriteString(fmt.Sprintf("  - Static methods [%d] {\n", staticMethodCount))
	for _, m := range zc.Methods {
		if !m.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatMethodShort(zc, m))
	}
	sb.WriteString("  }\n\n")

	nonStaticProps := 0
	for _, prop := range zc.Props {
		if !prop.Modifiers.IsStatic() {
			nonStaticProps++
		}
	}
	sb.WriteString(fmt.Sprintf("  - Properties [%d] {\n", nonStaticProps))
	for _, prop := range zc.Props {
		if prop.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatProperty(ctx, prop))
	}
	sb.WriteString("  }\n\n")

	nonStaticMethods := 0
	for _, m := range zc.Methods {
		if !m.Modifiers.IsStatic() {
			nonStaticMethods++
		}
	}
	sb.WriteString(fmt.Sprintf("  - Methods [%d] {\n", nonStaticMethods))
	for _, m := range zc.Methods {
		if m.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatMethodShort(zc, m))
	}
	sb.WriteString("  }\n}\n")

	return sb.String()
}

// formatConstantValue formats a constant value for ReflectionClass::__toString() output.
func formatConstantValue(ctx phpv.Context, val *phpv.ZVal) string {
	if val == nil {
		return "NULL"
	}
	switch val.GetType() {
	case phpv.ZtNull:
		return "NULL"
	case phpv.ZtBool:
		if val.AsBool(ctx) {
			return "true"
		}
		return "false"
	case phpv.ZtInt:
		return fmt.Sprintf("%d", val.AsInt(ctx))
	case phpv.ZtFloat:
		return fmt.Sprintf("%g", val.AsFloat(ctx))
	case phpv.ZtString:
		return string(val.AsString(ctx))
	case phpv.ZtArray:
		return "Array"
	case phpv.ZtObject:
		return "Object"
	default:
		return val.String()
	}
}

// findMethodPrototype walks up the class hierarchy to find the earliest
// class/interface that declares the given method. Returns empty if no prototype found.
func findMethodPrototype(zc *phpobj.ZClass, methodNameLower phpv.ZString) phpv.ZString {
	// Check interfaces first - they are the earliest prototype
	for _, impl := range zc.Implementations {
		if _, ok := impl.GetMethod(methodNameLower); ok {
			return impl.GetName()
		}
	}

	// Walk up parent chain to find the earliest declaration
	var earliest phpv.ZString
	for cur := zc.Extends; cur != nil; {
		if _, ok := cur.GetMethod(methodNameLower); ok {
			// Check interfaces of this parent
			for _, impl := range cur.Implementations {
				if _, ok := impl.GetMethod(methodNameLower); ok {
					return impl.GetName()
				}
			}
			earliest = cur.GetName()
			// Keep going up
			parent := cur.GetParent()
			if phpv.IsNilClass(parent) {
				break
			}
			var ok2 bool
			cur, ok2 = parent.(*phpobj.ZClass)
			if !ok2 {
				break
			}
		} else {
			break
		}
	}
	return earliest
}

func rcAccessStr(mod phpv.ZObjectAttr) string {
	if mod.IsProtected() {
		return "protected"
	}
	if mod.IsPrivate() {
		return "private"
	}
	return "public"
}

func rcFormatMethodShort(zc *phpobj.ZClass, m *phpv.ZClassMethod) string {
	var sb strings.Builder
	sb.WriteString("    Method [ ")
	origin := "<user"
	if m.Loc == nil {
		origin = "<internal"
	}
	methodNameLower := m.Name.ToLower()
	isOwnMethod := m.Class == nil || m.Class.GetName() == zc.GetName()

	if isOwnMethod && zc.Extends != nil {
		// Method is defined in this class - check if parent also has it ("overwrites")
		if parentMethod, ok := zc.Extends.GetMethod(methodNameLower); ok {
			// "overwrites" shows the class that actually declares the method
			declaringClass := zc.Extends.GetName()
			if parentMethod.Class != nil {
				declaringClass = parentMethod.Class.GetName()
			}
			origin += ", overwrites " + string(declaringClass)
		}
	}

	// Find prototype: walk up the full hierarchy to find the earliest declaration
	if isOwnMethod {
		protoName := findMethodPrototype(zc, methodNameLower)
		if protoName != "" {
			origin += ", prototype " + string(protoName)
		}
	} else if m.Prototype != nil {
		origin += ", prototype " + string(m.Prototype.GetName())
	}

	if !isOwnMethod {
		// Method is inherited
		declaringClass := m.Class.GetName()
		origin += ", inherits " + string(declaringClass)
		// Find prototype for inherited method
		if m.Prototype != nil {
			origin += ", prototype " + string(m.Prototype.GetName())
		} else {
			protoName := findMethodPrototype(zc, methodNameLower)
			if protoName != "" {
				origin += ", prototype " + string(protoName)
			}
		}
	}
	// Check if this is a constructor
	nameLower := strings.ToLower(string(m.Name))
	if nameLower == "__construct" {
		origin += ", ctor"
	} else if nameLower == "__destruct" {
		origin += ", dtor"
	}
	origin += ">"
	sb.WriteString(origin)
	if m.Modifiers.Has(phpv.ZAttrAbstract) || m.Empty {
		sb.WriteString(" abstract")
	}
	if m.Modifiers.Has(phpv.ZAttrFinal) {
		sb.WriteString(" final")
	}
	if m.Modifiers.IsStatic() {
		sb.WriteString(" static")
	}
	if m.Modifiers.IsProtected() {
		sb.WriteString(" protected")
	} else if m.Modifiers.IsPrivate() {
		sb.WriteString(" private")
	} else {
		sb.WriteString(" public")
	}
	sb.WriteString(fmt.Sprintf(" method %s ] {\n", m.Name))
	if m.Loc != nil {
		sb.WriteString(fmt.Sprintf("      @@ %s %d - %d\n", m.Loc.Filename, m.Loc.Line, m.Loc.Line))
	}
	if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
		funcArgs := fga.GetArgs()
		if len(funcArgs) > 0 {
			sb.WriteString(fmt.Sprintf("\n      - Parameters [%d] {\n", len(funcArgs)))
			for i, arg := range funcArgs {
				sb.WriteString(fmt.Sprintf("        Parameter #%d [ ", i))
				if !arg.Required {
					sb.WriteString("<optional> ")
				} else {
					sb.WriteString("<required> ")
				}
				if arg.Hint != nil {
					sb.WriteString(arg.Hint.String() + " ")
				}
				sb.WriteString(fmt.Sprintf("$%s", arg.VarName))
				sb.WriteString(" ]\n")
			}
			sb.WriteString("      }\n")
		}
	} else if m.Loc == nil {
		// Internal methods: show empty parameter list
		sb.WriteString(fmt.Sprintf("\n      - Parameters [0] {\n"))
		sb.WriteString("      }\n")
	}
	// Show return type if available
	if m.ReturnType != nil {
		sb.WriteString(fmt.Sprintf("      - Return [ %s ]\n", m.ReturnType.String()))
	}
	sb.WriteString("    }\n")
	return sb.String()
}

// rcFormatProperty formats a non-static property for ReflectionClass::__toString().
// Output format: "    Property [ public [protected(set)] [readonly] [type] $name [= default] ]\n"
func rcFormatProperty(ctx phpv.Context, prop *phpv.ZClassProp) string {
	var sb strings.Builder
	sb.WriteString("    Property [ ")
	sb.WriteString(rcAccessStr(prop.Modifiers))
	// Asymmetric set visibility (PHP 8.4)
	if prop.SetModifiers != 0 {
		setVis := "public"
		if prop.SetModifiers.IsProtected() {
			setVis = "protected"
		} else if prop.SetModifiers.IsPrivate() {
			setVis = "private"
		}
		sb.WriteString(" ")
		sb.WriteString(setVis)
		sb.WriteString("(set)")
	}
	if prop.Modifiers.Has(phpv.ZAttrReadonly) {
		sb.WriteString(" readonly")
	}
	if prop.TypeHint != nil {
		sb.WriteString(" " + prop.TypeHint.String())
	}
	sb.WriteString(fmt.Sprintf(" $%s", prop.VarName))
	// Show default value: always for untyped properties (defaults to NULL),
	// only when explicitly set for typed properties
	if prop.TypeHint == nil {
		// Untyped: always show default
		if prop.Default != nil {
			val := prop.Default
			if cd, ok := val.(*phpv.CompileDelayed); ok {
				resolved, err := cd.Run(ctx)
				if err == nil && resolved != nil {
					sb.WriteString(" = " + formatConstantValue(ctx, resolved))
				} else {
					sb.WriteString(" = NULL")
				}
			} else {
				sb.WriteString(" = " + formatConstantValue(ctx, val.ZVal()))
			}
		} else {
			sb.WriteString(" = NULL")
		}
	} else if prop.Default != nil {
		// Typed with default
		val := prop.Default
		if cd, ok := val.(*phpv.CompileDelayed); ok {
			resolved, err := cd.Run(ctx)
			if err == nil && resolved != nil {
				sb.WriteString(" = " + formatConstantValue(ctx, resolved))
			}
		} else {
			sb.WriteString(" = " + formatConstantValue(ctx, val.ZVal()))
		}
	}
	if prop.HasHooks {
		sb.WriteString(" {")
		if prop.GetHook != nil {
			sb.WriteString(" get;")
		}
		if prop.SetHook != nil {
			sb.WriteString(" set;")
		}
		sb.WriteString(" }")
	}
	sb.WriteString(" ]\n")
	return sb.String()
}

// rcFormatStaticProperty formats a static property for ReflectionClass::__toString().
func rcFormatStaticProperty(ctx phpv.Context, prop *phpv.ZClassProp) string {
	var sb strings.Builder
	sb.WriteString("    Property [ ")
	sb.WriteString(rcAccessStr(prop.Modifiers))
	sb.WriteString(" static")
	if prop.Modifiers.Has(phpv.ZAttrReadonly) {
		sb.WriteString(" readonly")
	}
	if prop.TypeHint != nil {
		sb.WriteString(" " + prop.TypeHint.String())
	}
	sb.WriteString(fmt.Sprintf(" $%s", prop.VarName))
	// Show default value: always for untyped properties, only when set for typed
	if prop.TypeHint == nil {
		if prop.Default != nil {
			val := prop.Default
			if cd, ok := val.(*phpv.CompileDelayed); ok {
				resolved, err := cd.Run(ctx)
				if err == nil && resolved != nil {
					sb.WriteString(" = " + formatConstantValue(ctx, resolved))
				} else {
					sb.WriteString(" = NULL")
				}
			} else {
				sb.WriteString(" = " + formatConstantValue(ctx, val.ZVal()))
			}
		} else {
			sb.WriteString(" = NULL")
		}
	} else if prop.Default != nil {
		val := prop.Default
		if cd, ok := val.(*phpv.CompileDelayed); ok {
			resolved, err := cd.Run(ctx)
			if err == nil && resolved != nil {
				sb.WriteString(" = " + formatConstantValue(ctx, resolved))
			}
		} else {
			sb.WriteString(" = " + formatConstantValue(ctx, val.ZVal()))
		}
	}
	sb.WriteString(" ]\n")
	return sb.String()
}

// --- Additional methods for ReflectionMethod ---

func reflectionMethodGetReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	type returnTypeGetter interface {
		GetReturnType() *phpv.TypeHint
	}
	if rtg, ok := data.method.Method.(returnTypeGetter); ok {
		rt := rtg.GetReturnType()
		if rt != nil {
			return createReflectionTypeObject(ctx, rt)
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

func reflectionMethodHasReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type returnTypeGetter interface {
		GetReturnType() *phpv.TypeHint
	}
	if rtg, ok := data.method.Method.(returnTypeGetter); ok {
		if rtg.GetReturnType() != nil {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodIsDeprecated(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if data.method.Modifiers.Has(phpv.ZAttrDeprecated) {
		return phpv.ZBool(true).ZVal(), nil
	}
	// Check for #[Deprecated] attribute
	for _, attr := range data.method.Attributes {
		if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodHasPrototype(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// A method has a prototype if it overrides a method from a parent class or interface
	methodNameLower := data.method.Name.ToLower()

	zc, ok := data.class.(*phpobj.ZClass)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Check parent classes
	if zc.Extends != nil {
		if _, ok := zc.Extends.GetMethod(methodNameLower); ok {
			return phpv.ZBool(true).ZVal(), nil
		}
	}

	// Check interfaces
	for _, impl := range zc.Implementations {
		if _, ok := impl.GetMethod(methodNameLower); ok {
			return phpv.ZBool(true).ZVal(), nil
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

func reflectionMethodCreateFromMethodName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionMethod::createFromMethodName() expects exactly 1 argument")
	}
	methodStr := string(args[0].AsString(ctx))
	parts := strings.SplitN(methodStr, "::", 2)
	if len(parts) != 2 {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("ReflectionMethod::createFromMethodName(): Argument #1 ($method) must be a valid method name"))
	}
	class, err := resolveClass(ctx, phpv.ZString(parts[0]))
	if err != nil {
		return nil, err
	}
	method, ok := class.GetMethod(phpv.ZString(parts[1]))
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Method %s::%s() does not exist", parts[0], parts[1]))
	}
	return createReflectionMethodObject(ctx, class, method)
}

// --- Additional methods for ReflectionProperty ---

func reflectionPropertyGetType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil || data.prop.TypeHint == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionTypeObject(ctx, data.prop.TypeHint)
}

func reflectionPropertyHasType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil || data.prop.TypeHint == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionPropertyHasDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Typed properties without an explicit default don't have a default value
	if data.prop.Default == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func reflectionPropertyGetDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if data.prop.Default == nil {
		// PHP 8.5: return NULL with a deprecation notice instead of throwing
		_ = ctx.Deprecated("ReflectionProperty::getDefaultValue() for a property without a default value is deprecated, use ReflectionProperty::hasDefaultValue() to check if the default value exists", logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}
	// Resolve CompileDelayed values
	if cd, ok := data.prop.Default.(*phpv.CompileDelayed); ok {
		resolved, err := cd.Run(ctx)
		if err != nil {
			return nil, err
		}
		return resolved, nil
	}
	return data.prop.Default.ZVal(), nil
}

func reflectionPropertyIsReadOnly(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.prop.Modifiers.IsReadonly()).ZVal(), nil
}

func reflectionPropertyGetModifiers(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	var mods int64
	if data.prop.Modifiers.IsProtected() {
		mods |= ReflectionMethodIS_PROTECTED
	} else if data.prop.Modifiers.IsPrivate() {
		mods |= ReflectionMethodIS_PRIVATE
	} else {
		mods |= ReflectionMethodIS_PUBLIC
	}
	if data.prop.Modifiers.IsStatic() {
		mods |= ReflectionMethodIS_STATIC
	}
	if data.prop.Modifiers.IsReadonly() {
		mods |= 128
	}
	if data.prop.Modifiers.Has(phpv.ZAttrAbstract) {
		mods |= ReflectionMethodIS_ABSTRACT
	}
	if data.prop.Modifiers.Has(phpv.ZAttrFinal) {
		mods |= ReflectionMethodIS_FINAL
	}
	// IS_VIRTUAL = 512
	if data.prop.HasHooks && !data.prop.IsBacked {
		mods |= 512
	}
	return phpv.ZInt(mods).ZVal(), nil
}

// --- Additional methods for ReflectionParameter ---

func reflectionParameterIsDefaultValueAvailable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.arg.DefaultValue != nil).ZVal(), nil
}

func reflectionParameterToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZString("Parameter #0 [ ]").ZVal(), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Parameter #%d [ ", data.position))
	if !data.arg.Required {
		sb.WriteString("<optional> ")
	} else {
		sb.WriteString("<required> ")
	}
	if data.arg.Hint != nil {
		sb.WriteString(data.arg.Hint.String() + " ")
	}
	sb.WriteString(fmt.Sprintf("$%s", data.arg.VarName))
	if data.arg.DefaultValue != nil {
		if cd, ok := data.arg.DefaultValue.(*phpv.CompileDelayed); ok {
			resolved, err := cd.Run(ctx)
			if err == nil && resolved != nil {
				sb.WriteString(fmt.Sprintf(" = %s", resolved.String()))
			}
		} else {
			sb.WriteString(fmt.Sprintf(" = %s", data.arg.DefaultValue.String()))
		}
	}
	sb.WriteString(" ]")
	return phpv.ZString(sb.String()).ZVal(), nil
}

func reflectionParameterGetDeclaringFunction(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getParamData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if strings.Contains(string(data.funcName), "::") {
		parts := strings.SplitN(string(data.funcName), "::", 2)
		class, err := ctx.Global().GetClass(ctx, phpv.ZString(parts[0]), false)
		if err == nil {
			method, ok := class.GetMethod(phpv.ZString(parts[1]))
			if ok {
				return createReflectionMethodObject(ctx, class, method)
			}
		}
	}
	rfObj, err := phpobj.CreateZObject(ctx, ReflectionFunction)
	if err != nil {
		return nil, err
	}
	rfObj.HashTable().SetString("name", data.funcName.ZVal())
	fn, fnErr := ctx.Global().GetFunction(ctx, data.funcName)
	if fnErr == nil {
		fData := &reflectionFunctionData{
			name:     data.funcName,
			callable: fn,
		}
		if fga, ok := fn.(phpv.FuncGetArgs); ok {
			fData.args = fga.GetArgs()
		}
		rfObj.SetOpaque(ReflectionFunction, fData)
	}
	return rfObj.ZVal(), nil
}

// --- Additional methods for ReflectionFunction ---

func reflectionFunctionIsDeprecated(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Check for #[Deprecated] attribute
	type attrGetter interface {
		GetAttributes() []*phpv.ZAttribute
	}
	if ag, ok := data.callable.(attrGetter); ok {
		for _, attr := range ag.GetAttributes() {
			if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
				return phpv.ZBool(true).ZVal(), nil
			}
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionGetExtensionName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionIsVariadic(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.args == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	for _, arg := range data.args {
		if arg.Variadic {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionIsAnonymous(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.closure != nil).ZVal(), nil
}

func reflectionFunctionGetFileName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type locGetter interface {
		Loc() *phpv.Loc
	}
	if lg, ok := data.callable.(locGetter); ok {
		loc := lg.Loc()
		if loc != nil && loc.Filename != "" {
			return phpv.ZString(loc.Filename).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionGetStaticVariables(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.NewZArray().ZVal(), nil
}

func reflectionFunctionIsGenerator(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type generatorChecker interface {
		IsGenerator() bool
	}
	if gc, ok := data.callable.(generatorChecker); ok {
		return phpv.ZBool(gc.IsGenerator()).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionIsDisabled(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// No function disabling mechanism in goro
	// In PHP 8.0+, this is deprecated and always returns false
	_ = ctx.Deprecated("Method ReflectionFunction::isDisabled() is deprecated since 8.0, as ReflectionFunction can no longer be constructed for disabled functions", logopt.NoFuncName(true))
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func reflectionFunctionGetClosureCalledClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil || data.closure == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	class := data.closure.GetClass()
	if class == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return createReflectionClassObject(ctx, class)
}

func reflectionFunctionReturnsReference(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type refGetter interface {
		ReturnsRef() bool
	}
	if rg, ok := data.callable.(refGetter); ok {
		return phpv.ZBool(rg.ReturnsRef()).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZString("Function [ ]").ZVal(), nil
	}

	var sb strings.Builder
	origin := "<user>"

	if data.closure != nil {
		sb.WriteString(fmt.Sprintf("Closure [ %s closure %s ] {\n", origin, data.name))
	} else {
		sb.WriteString(fmt.Sprintf("Function [ %s function %s ] {\n", origin, data.name))
	}

	if data.args != nil && len(data.args) > 0 {
		sb.WriteString(fmt.Sprintf("\n  - Parameters [%d] {\n", len(data.args)))
		for i, arg := range data.args {
			sb.WriteString(fmt.Sprintf("    Parameter #%d [ ", i))
			if !arg.Required {
				sb.WriteString("<optional> ")
			} else {
				sb.WriteString("<required> ")
			}
			if arg.Hint != nil {
				sb.WriteString(arg.Hint.String() + " ")
			}
			sb.WriteString(fmt.Sprintf("$%s", arg.VarName))
			sb.WriteString(" ]\n")
		}
		sb.WriteString("  }\n")
	}
	sb.WriteString("}\n")

	return phpv.ZString(sb.String()).ZVal(), nil
}

func reflectionFunctionGetStartLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type locGetter interface {
		Loc() *phpv.Loc
	}
	if lg, ok := data.callable.(locGetter); ok {
		loc := lg.Loc()
		if loc != nil {
			return phpv.ZInt(loc.Line).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionGetEndLine(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type locGetter interface {
		Loc() *phpv.Loc
	}
	if lg, ok := data.callable.(locGetter); ok {
		loc := lg.Loc()
		if loc != nil {
			return phpv.ZInt(loc.Line).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionHasReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type returnTypeGetter interface {
		GetReturnType() *phpv.TypeHint
	}
	if rtg, ok := data.callable.(returnTypeGetter); ok {
		if rtg.GetReturnType() != nil {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

// --- Additional methods for ReflectionClassConstant ---

func reflectionClassConstantIsDeprecated(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	for _, attr := range data.constVal.Attributes {
		if attr.ClassName == "Deprecated" || attr.ClassName.ToLower() == "deprecated" {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

// --- Additional methods for ReflectionConstant ---

func reflectionConstantIsDeprecated(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	attrs := ctx.Global().ConstantGetAttributes(data.name)
	for _, attr := range attrs {
		if attr.ClassName == "Deprecated" || attr.ClassName.ToLower() == "deprecated" {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionConstantGetExtensionName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionConstantGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

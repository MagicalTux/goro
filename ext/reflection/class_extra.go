package reflection

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

func reflectionClassToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.ZString("").ZVal(), nil
	}
	// Check if this is a ReflectionObject (uses "Object of class" prefix)
	isReflectionObj := o.GetClass() == ReflectionObject
	var dynamicProps []phpv.ZString
	if isReflectionObj {
		// Collect dynamic (non-declared) public properties from the reflected object instance.
		if instanceOpaque := o.GetOpaque(ReflectionObject); instanceOpaque != nil {
			if instance, ok := instanceOpaque.(phpv.ZObject); ok {
				dynamicProps = collectDynamicProps(zc, instance)
			}
		}
	}
	s, err := formatReflectionClass(ctx, zc, isReflectionObj, dynamicProps)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(s).ZVal(), nil
}

// collectDynamicProps returns the names of properties on obj that are not declared in the class.
// Only public dynamic properties are included (matching PHP's behavior).
// This is used for __toString display - it treats ALL properties in the hierarchy
// as "declared", so dynamic props with the same name as private parent props are NOT shown.
func collectDynamicProps(zc *phpobj.ZClass, obj phpv.ZObject) []phpv.ZString {
	// Build a set of declared property names (from all levels of the hierarchy).
	declared := make(map[phpv.ZString]bool)
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			declared[prop.VarName] = true
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		if pc, ok := parent.(*phpobj.ZClass); ok {
			cur = pc
		} else {
			break
		}
	}
	// Iterate the object's hash table and find entries not in declared
	var result []phpv.ZString
	ht := obj.HashTable()
	if ht == nil {
		return result
	}
	it := ht.NewIterator()
	for keyVal, _ := range it.Iterate(nil) {
		if keyVal == nil {
			continue
		}
		if keyVal.GetType() != phpv.ZtString {
			continue
		}
		name := keyVal.AsString(nil)
		// Skip mangled property names (private/protected props stored with mangled keys
		// like "*ClassName:propName" or similar internal representations)
		if strings.HasPrefix(string(name), "*") || strings.Contains(string(name), "\x00") {
			continue
		}
		if !declared[name] {
			result = append(result, name)
		}
	}
	return result
}

// collectDynamicPropsForGetProps returns the names of dynamic properties on obj,
// similar to collectDynamicProps but used for getProperties(). Unlike collectDynamicProps,
// private props from parent classes are NOT considered "declared", so dynamic properties
// with the same name as a parent's private property ARE included.
func collectDynamicPropsForGetProps(zc *phpobj.ZClass, obj phpv.ZObject) []phpv.ZString {
	// Build declared set - exclude private props from parent classes.
	declared := make(map[phpv.ZString]bool)
	for cur := zc; cur != nil; {
		isChild := cur == zc
		for _, prop := range cur.Props {
			if !isChild && prop.Modifiers.IsPrivate() {
				// Private props from parent classes use mangled keys; they don't
				// prevent a dynamic prop with the same unmangled name.
				continue
			}
			declared[prop.VarName] = true
		}
		parent := cur.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
		if pc, ok := parent.(*phpobj.ZClass); ok {
			cur = pc
		} else {
			break
		}
	}
	// Iterate the object's hash table and find entries not in declared
	var result []phpv.ZString
	ht := obj.HashTable()
	if ht == nil {
		return result
	}
	it := ht.NewIterator()
	for keyVal, _ := range it.Iterate(nil) {
		if keyVal == nil {
			continue
		}
		if keyVal.GetType() != phpv.ZtString {
			continue
		}
		name := keyVal.AsString(nil)
		if strings.HasPrefix(string(name), "*") || strings.Contains(string(name), "\x00") {
			continue
		}
		if !declared[name] {
			result = append(result, name)
		}
	}
	return result
}

// collectVisibleProps returns all properties visible for a class:
// the class's own properties (including private) plus inherited public/protected
// from parent classes (not private inherited).
// Properties are returned in the same order as PHP's reflection: own props first,
// then parent props (excluding private).
func collectVisibleProps(zc *phpobj.ZClass) []*phpv.ZClassProp {
	// Collect own props + inherited public/protected from parents
	var result []*phpv.ZClassProp
	seen := make(map[phpv.ZString]bool)

	// Own properties first
	for _, prop := range zc.Props {
		seen[prop.VarName] = true
		result = append(result, prop)
	}

	// Walk up the hierarchy and include public/protected (not private) props
	for parent := zc.GetParent(); !phpv.IsNilClass(parent); {
		pc, ok := parent.(*phpobj.ZClass)
		if !ok {
			break
		}
		for _, prop := range pc.Props {
			if seen[prop.VarName] {
				continue
			}
			// Skip private props from parent classes
			if prop.Modifiers.IsPrivate() {
				continue
			}
			seen[prop.VarName] = true
			result = append(result, prop)
		}
		parent = pc.GetParent()
		if phpv.IsNilClass(parent) {
			break
		}
	}
	return result
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
	var resolved *phpv.ZVal
	if cd, ok := constVal.Value.(*phpv.CompileDelayed); ok {
		var err error
		resolved, err = cd.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		resolved = constVal.Value.ZVal()
	}
	if constVal.TypeHint != nil && resolved != nil && !constVal.TypeHint.CheckStrict(ctx, resolved) {
		typeName := classConstValueTypeName(ctx, resolved)
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
				typeName, zc.GetName(), name, constVal.TypeHint.String()))
	}
	return resolved, nil
}

func reflectionClassGetDefaultProperties(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o)
	if zc == nil {
		return phpv.NewZArray().ZVal(), nil
	}
	// PHP returns static properties first (all classes), then instance properties (all classes).
	// Within each group, properties are ordered by class hierarchy (the class itself first, then parents).
	// We collect static and instance props separately, then merge.
	arr := phpv.NewZArray()
	seenStatic := make(map[string]bool)
	seenInstance := make(map[string]bool)

	// Collect all static props first (class hierarchy order)
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			if !prop.Modifiers.IsStatic() {
				continue
			}
			key := string(prop.VarName)
			if seenStatic[key] {
				continue
			}
			if cur != zc && prop.Modifiers.IsPrivate() {
				continue
			}
			seenStatic[key] = true
			val := prop.Default
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
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

	// Then collect all instance props (class hierarchy order)
	for cur := zc; cur != nil; {
		for _, prop := range cur.Props {
			if prop.Modifiers.IsStatic() {
				continue
			}
			key := string(prop.VarName)
			if seenInstance[key] {
				continue
			}
			if cur != zc && prop.Modifiers.IsPrivate() {
				continue
			}
			seenInstance[key] = true
			val := prop.Default
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
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
	arr := phpv.NewZArray()
	seen := make(map[string]bool)
	for cur := zc; cur != nil; {
		staticProps, err := cur.GetStaticProps(ctx)
		if err != nil {
			return nil, err
		}
		if staticProps != nil {
			it := staticProps.NewIterator()
			for it.Valid(ctx) {
				k, _ := it.Key(ctx)
				v, _ := it.Current(ctx)
				key := ""
				if k.GetType() == phpv.ZtString {
					key = string(k.Value().(phpv.ZString))
				}
				// Skip private properties from parent classes and already seen props
				if key != "" && !seen[key] {
					// Check if this prop is private in a parent class
					if cur != zc {
						// Find the declaration to check visibility
						isPrivate := false
						for _, prop := range cur.Props {
							if prop.VarName == phpv.ZString(key) && prop.Modifiers.IsPrivate() {
								isPrivate = true
								break
							}
						}
						if isPrivate {
							it.Next(ctx)
							continue
						}
					}
					seen[key] = true
					if v != nil {
						arr.OffsetSet(ctx, k.Value(), v)
					}
				}
				it.Next(ctx)
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
			ctorMethod := zc.Handlers().Constructor
			hasConstructor = true
			if ctorMethod.Modifiers.IsPrivate() || ctorMethod.Modifiers.IsProtected() {
				return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Access to non-public constructor of class %s", class.GetName()))
			}
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

	// Suppress "called in X on line Y" in constructor errors - when called via
	// reflection, PHP does not include the call site in argument count errors.
	ctx.Global().SetNextCallSuppressCalledIn(true)
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
	// Enums cannot be cloned
	if zc.Type.Has(phpv.ZClassTypeEnum) {
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
	if zc.LEnd != nil {
		return phpv.ZInt(zc.LEnd.Line).ZVal(), nil
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
		// Collect directly declared/implemented interfaces
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
		// For interfaces, also collect the parent interface (stored in Extends/parent)
		parent := c.GetParent()
		if !phpv.IsNilClass(parent) {
			if pc, ok := parent.(*phpobj.ZClass); ok {
				// If parent is an interface, add it and recurse
				if pc.Type == phpv.ZClassTypeInterface {
					key := strings.ToLower(string(pc.GetName()))
					if !seen[key] {
						seen[key] = true
						rcVal, err := createReflectionClassObject(ctx, pc)
						if err == nil {
							arr.OffsetSet(ctx, pc.GetName(), rcVal)
						}
					}
					collectInterfaces(pc)
				} else {
					// Regular parent class - recurse to collect its interfaces
					collectInterfaces(pc)
				}
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
// isObject: if true, use "Object of class [" prefix (ReflectionObject::__toString)
// dynamicProps: optional list of dynamic property names to show in the output (ReflectionObject only)
func formatReflectionClass(ctx phpv.Context, zc *phpobj.ZClass, isObject bool, dynamicProps []phpv.ZString) (string, error) {
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
		if zc.Ext != "" {
			origin = "<internal:" + zc.Ext + ">"
		} else {
			origin = "<internal:Core>"
		}
	}

	iterateable := ""
	traversable, err := ctx.Global().GetClass(ctx, "Traversable", false)
	if err == nil {
		var class phpv.ZClass = zc
		if class.InstanceOf(traversable) {
			iterateable = " <iterateable>"
		}
	}
	// PHP 8.4: classes with property hooks are also shown as <iterateable>
	if iterateable == "" {
		for _, prop := range zc.Props {
			if prop.HasHooks {
				iterateable = " <iterateable>"
				break
			}
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

	if isObject {
		sb.WriteString(fmt.Sprintf("Object of class [ %s%s%s %s %s",
			origin, iterateable, modifiers, kindLower, string(zc.GetName())))
	} else {
		sb.WriteString(fmt.Sprintf("%s [ %s%s%s %s %s",
			kind, origin, iterateable, modifiers, kindLower, string(zc.GetName())))
	}

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
		endLine := zc.L.Line
		if zc.LEnd != nil {
			endLine = zc.LEnd.Line
		}
		sb.WriteString(fmt.Sprintf("  @@ %s %d-%d\n", zc.L.Filename, zc.L.Line, endLine))
	}
	sb.WriteString("\n")

	// Build a set of enum case names for quick lookup
	enumCaseSet := make(map[phpv.ZString]bool)
	for _, caseName := range zc.EnumCases {
		enumCaseSet[caseName] = true
	}

	// Count non-enum-case constants
	constCount := 0
	if zc.Const != nil {
		for name := range zc.Const {
			if !enumCaseSet[name] {
				constCount++
			}
		}
	}
	sb.WriteString(fmt.Sprintf("  - Constants [%d] {\n", constCount))
	if zc.Const != nil {
		for _, name := range zc.ConstOrder {
			// Skip enum cases - they are shown in "Enum cases" section
			if enumCaseSet[name] {
				continue
			}
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
			var resolvedVal *phpv.ZVal
			if c.Value != nil {
				if cd, ok := c.Value.(*phpv.CompileDelayed); ok {
					resolved, resolveErr := cd.Run(ctx)
					if resolveErr != nil {
						return "", resolveErr
					}
					if resolved != nil {
						resolvedVal = resolved
						valStr = formatConstantValue(ctx, resolved)
					}
				} else {
					resolvedVal = c.Value.ZVal()
					valStr = formatConstantValue(ctx, resolvedVal)
				}
			}
			// Validate type if there's a type hint
			if c.TypeHint != nil && resolvedVal != nil && !c.TypeHint.CheckStrict(ctx, resolvedVal) {
				typeName := classConstValueTypeName(ctx, resolvedVal)
				return "", phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
						typeName, zc.GetName(), name, c.TypeHint.String()))
			}
			// Infer type from value when no explicit TypeHint
			if c.TypeHint == nil && resolvedVal != nil {
				typeStr = inferTypeFromValue(resolvedVal)
			}
			sb.WriteString(fmt.Sprintf("    Constant [ %s %s %s ] { %s }\n", modStr, typeStr, name, valStr))
		}
	}
	sb.WriteString("  }\n\n")

	// Collect static props from this class and inherited public/protected static props from parents
	visibleStaticProps := collectVisibleProps(zc)
	staticCount := 0
	for _, prop := range visibleStaticProps {
		if prop.Modifiers.IsStatic() {
			staticCount++
		}
	}
	sb.WriteString(fmt.Sprintf("  - Static properties [%d] {\n", staticCount))
	for _, prop := range visibleStaticProps {
		if !prop.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatStaticProperty(ctx, prop))
	}
	sb.WriteString("  }\n\n")

	// Get all methods in declaration order
	allMethods := zc.GetMethodsOrdered()

	// Count static methods (excluding private inherited)
	staticMethodCount := 0
	for _, m := range allMethods {
		if !m.Modifiers.IsStatic() {
			continue
		}
		// Skip private methods from parent classes
		if m.Class != nil && m.Class.GetName() != zc.GetName() && m.Modifiers.IsPrivate() {
			continue
		}
		staticMethodCount++
	}
	sb.WriteString(fmt.Sprintf("  - Static methods [%d] {\n", staticMethodCount))
	firstStaticMethod := true
	for _, m := range allMethods {
		if !m.Modifiers.IsStatic() {
			continue
		}
		if m.Class != nil && m.Class.GetName() != zc.GetName() && m.Modifiers.IsPrivate() {
			continue
		}
		if !firstStaticMethod {
			sb.WriteString("\n")
		}
		firstStaticMethod = false
		sb.WriteString(rcFormatMethodShort(ctx, zc, m))
	}
	sb.WriteString("  }\n\n")

	// Collect all non-static properties visible for this class:
	// own properties + inherited public/protected (not private) from parents.
	visibleProps := collectVisibleProps(zc)
	nonStaticProps := 0
	for _, prop := range visibleProps {
		if !prop.Modifiers.IsStatic() {
			nonStaticProps++
		}
	}
	isReadonlyClass := zc.Attr.Has(phpv.ZClassReadonly)
	isInterface := zc.Type.Has(phpv.ZClassTypeInterface)
	sb.WriteString(fmt.Sprintf("  - Properties [%d] {\n", nonStaticProps))
	for _, prop := range visibleProps {
		if prop.Modifiers.IsStatic() {
			continue
		}
		sb.WriteString(rcFormatProperty(ctx, prop, isReadonlyClass, isInterface))
	}
	sb.WriteString("  }\n\n")

	// Dynamic properties section (ReflectionObject only, shown even if empty)
	if isObject {
		sb.WriteString(fmt.Sprintf("  - Dynamic properties [%d] {\n", len(dynamicProps)))
		for _, name := range dynamicProps {
			sb.WriteString(fmt.Sprintf("    Property [ <dynamic> public $%s ]\n", name))
		}
		sb.WriteString("  }\n\n")
	}

	// Count non-static methods (excluding private inherited)
	nonStaticMethods := 0
	for _, m := range allMethods {
		if m.Modifiers.IsStatic() {
			continue
		}
		if m.Class != nil && m.Class.GetName() != zc.GetName() && m.Modifiers.IsPrivate() {
			continue
		}
		nonStaticMethods++
	}
	sb.WriteString(fmt.Sprintf("  - Methods [%d] {\n", nonStaticMethods))
	firstNonStaticMethod := true
	for _, m := range allMethods {
		if m.Modifiers.IsStatic() {
			continue
		}
		if m.Class != nil && m.Class.GetName() != zc.GetName() && m.Modifiers.IsPrivate() {
			continue
		}
		if !firstNonStaticMethod {
			sb.WriteString("\n")
		}
		firstNonStaticMethod = false
		sb.WriteString(rcFormatMethodShort(ctx, zc, m))
	}
	sb.WriteString("  }\n}\n")

	return sb.String(), nil
}

// inferTypeFromValue infers a PHP type name string from a ZVal for constant type display.
func inferTypeFromValue(val *phpv.ZVal) string {
	if val == nil {
		return "mixed"
	}
	switch val.GetType() {
	case phpv.ZtNull:
		return "null"
	case phpv.ZtBool:
		return "bool"
	case phpv.ZtInt:
		return "int"
	case phpv.ZtFloat:
		return "float"
	case phpv.ZtString:
		return "string"
	case phpv.ZtArray:
		return "array"
	default:
		return "mixed"
	}
}

// phpEscapeSingleQuote escapes single quotes and backslashes for PHP single-quoted string literals.
func phpEscapeSingleQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

// isConstantIdentifier returns true if s looks like a PHP constant name
// (an identifier possibly with namespace backslashes), not a string literal,
// number, array, or other expression (e.g. "new Foo()" is NOT a constant).
func isConstantIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch strings.ToLower(s) {
	case "null", "true", "false":
		return false
	}
	// A valid constant name only contains letters, digits, underscores, and backslashes.
	// No spaces, no parens, no quotes.
	for _, ch := range s {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '\\') {
			return false
		}
	}
	// Must start with a letter, underscore, or backslash (not a digit)
	ch := s[0]
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '\\'
}

// formatParamDefault returns the string representation of a parameter default value
// for use in reflection __toString output (e.g. " = null", " = 'hello'", " = K").
func formatParamDefault(ctx phpv.Context, arg *phpv.FuncArg) string {
	if arg.DefaultValue == nil {
		if !arg.Required && !arg.Variadic {
			// Optional non-variadic parameter with no explicit default: show " = <default>"
			return " = <default>"
		}
		return ""
	}
	// If we have a preserved expression string (from before Compile() evaluated it),
	// use it only if it's a constant name. For other expressions (strings, numbers),
	// we fall through to use the evaluated value (which Compile() has resolved).
	if arg.DefaultValueExpr != "" {
		s := arg.DefaultValueExpr
		if strings.EqualFold(s, "null") {
			// null is handled below via the evaluated value
		} else if isConstantIdentifier(s) {
			// This is a constant name like "foobar" or "MyClass::CONST"
			return " = " + s
		}
	}
	cd, ok := arg.DefaultValue.(*phpv.CompileDelayed)
	if !ok {
		// Direct value (already evaluated at compile time by Compile())
		val := arg.DefaultValue.ZVal()
		// For internal methods (no DefaultValueExpr), null defaults are shown as lowercase "null".
		// For user-defined code, formatParamValue returns uppercase "NULL" (handled above via CompileDelayed).
		if arg.DefaultValueExpr == "" && val.GetType() == phpv.ZtNull {
			return " = null"
		}
		return " = " + formatParamValue(ctx, val)
	}
	// Use Dump() to get the PHP source representation (preserves constant names, quoted strings)
	var buf bytes.Buffer
	if err := cd.V.Dump(&buf); err == nil {
		s := buf.String()
		if s == "" {
			return " = NULL"
		}
		// For user-defined code, PHP reflection uses uppercase NULL for null defaults
		if strings.EqualFold(s, "null") {
			return " = NULL"
		}
		return " = " + s
	}
	// Fallback: evaluate and format
	if resolved, err := cd.Run(ctx); err == nil && resolved != nil {
		return " = " + formatParamValue(ctx, resolved)
	}
	return " = <default>"
}

// formatParamValue formats an evaluated value for parameter default display.
// Like formatConstantValue but always single-quotes strings (matching PHP reflection output).
func formatParamValue(ctx phpv.Context, val *phpv.ZVal) string {
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
		return "'" + phpEscapeSingleQuote(string(val.AsString(ctx))) + "'"
	case phpv.ZtArray:
		// PHP reflection shows [] for empty arrays in parameter defaults
		arr := val.Array()
		if arr == nil {
			return "[]"
		}
		if cnt, ok := arr.(phpv.ZCountable); ok && cnt.Count(ctx) == 0 {
			return "[]"
		}
		return "Array"
	case phpv.ZtObject:
		obj := val.AsObject(ctx)
		if obj != nil {
			return fmt.Sprintf("object(%s)", obj.GetClass().GetName())
		}
		return "Object"
	default:
		return val.String()
	}
}

// formatConstantValue formats a constant value for ReflectionClass::__toString() output.
func formatConstantValue(ctx phpv.Context, val *phpv.ZVal) string {
	if val == nil {
		return ""
	}
	switch val.GetType() {
	case phpv.ZtNull:
		return "NULL"
	case phpv.ZtBool:
		// PHP reflection uses integer string representation for booleans in constant values
		if val.AsBool(ctx) {
			return "1"
		}
		return ""
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

func rcFormatMethodShort(ctx phpv.Context, zc *phpobj.ZClass, m *phpv.ZClassMethod) string {
	var sb strings.Builder
	sb.WriteString("    Method [ ")
	methodNameLower := m.Name.ToLower()
	isOwnMethod := m.Class == nil || m.Class.GetName() == zc.GetName()

	// Determine the origin prefix
	isInternal := m.Loc == nil
	origin := "<user"
	if isInternal {
		// Use the method's own class extension if available, then fall back to the current class's extension.
		// For methods added to user-defined classes (zc.L != nil), use plain <internal> without suffix.
		methodExt := ""
		if m.Class != nil {
			if mc, ok := m.Class.(*phpobj.ZClass); ok {
				methodExt = mc.Ext
			}
		}
		if methodExt == "" {
			methodExt = zc.Ext
		}
		if methodExt != "" {
			origin = "<internal:" + methodExt
		} else if zc.L != nil {
			// User-defined class with internal method (e.g., enum built-ins) — no extension suffix
			origin = "<internal"
		} else {
			origin = "<internal:Core"
		}
	}

	if isOwnMethod {
		// Private methods do NOT show "overwrites" or "prototype" - they are not polymorphic
		if !m.Modifiers.IsPrivate() && zc.Extends != nil {
			// Method is defined in this class - check if parent also has it ("overwrites")
			if parentMethod, ok := zc.Extends.GetMethod(methodNameLower); ok {
				// Only show "overwrites" if the parent method is NOT private
				if !parentMethod.Modifiers.IsPrivate() {
					// "overwrites" shows the class that actually declares the method in the parent chain
					declaringClass := zc.Extends.GetName()
					if parentMethod.Class != nil {
						declaringClass = parentMethod.Class.GetName()
					}
					origin += ", overwrites " + string(declaringClass)
				}
			}
		}
		// Find prototype: walk up the full hierarchy to find the earliest declaration
		// Private methods have no prototype
		if !m.Modifiers.IsPrivate() {
			var protoName phpv.ZString
			if m.Prototype != nil {
				protoName = m.Prototype.GetName()
			} else {
				protoName = findMethodPrototype(zc, methodNameLower)
			}
			if protoName != "" {
				origin += ", prototype " + string(protoName)
			}
		}
	} else {
		// Method is inherited from another class
		declaringClass := m.Class.GetName()
		origin += ", inherits " + string(declaringClass)
		// Find prototype for inherited method - only show if different from declaring class
		// Private methods have no prototype
		if !m.Modifiers.IsPrivate() {
			var protoName phpv.ZString
			if m.Prototype != nil {
				protoName = m.Prototype.GetName()
			} else {
				protoName = findMethodPrototype(zc, methodNameLower)
			}
			if protoName != "" && protoName != declaringClass {
				origin += ", prototype " + string(protoName)
			}
		}
	}

	// Check if this is a constructor (PHP shows ctor but not dtor in reflection output)
	nameLower := strings.ToLower(string(m.Name))
	if nameLower == "__construct" {
		origin += ", ctor"
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
		endLine := m.Loc.Line
		if m.LocEnd != nil {
			endLine = m.LocEnd.Line
		} else {
			// Try to get end line from the callable
			type locEndGetter interface {
				LocEnd() *phpv.Loc
			}
			if leg, ok := m.Method.(locEndGetter); ok {
				if loc := leg.LocEnd(); loc != nil {
					endLine = loc.Line
				}
			}
		}
		sb.WriteString(fmt.Sprintf("      @@ %s %d - %d\n", m.Loc.Filename, m.Loc.Line, endLine))
	}

	if fga, ok := m.Method.(phpv.FuncGetArgs); ok {
		funcArgs := fga.GetArgs()
		if len(funcArgs) > 0 || isInternal {
			sb.WriteString(fmt.Sprintf("\n      - Parameters [%d] {\n", len(funcArgs)))
			for i, arg := range funcArgs {
				sb.WriteString(rcFormatParameter(ctx, i, arg, "        "))
			}
			sb.WriteString("      }\n")
		}
	} else if isInternal {
		// Internal methods without FuncGetArgs: show empty parameter list
		sb.WriteString("\n      - Parameters [0] {\n")
		sb.WriteString("      }\n")
	}
	// Show return type if available
	if m.ReturnType != nil {
		if m.TentativeReturnType {
			sb.WriteString(fmt.Sprintf("      - Tentative return [ %s ]\n", m.ReturnType.String()))
		} else {
			sb.WriteString(fmt.Sprintf("      - Return [ %s ]\n", m.ReturnType.String()))
		}
	}
	sb.WriteString("    }\n")
	return sb.String()
}

// rcFormatParameter formats a single parameter for reflection output.
// Used by both rcFormatMethodShort and reflectionMethodToString.
func rcFormatParameter(ctx phpv.Context, idx int, arg *phpv.FuncArg, indent string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sParameter #%d [ ", indent, idx))
	if arg.Variadic {
		// Variadic parameters are always optional
		sb.WriteString("<optional> ")
	} else if !arg.Required {
		sb.WriteString("<optional> ")
	} else {
		sb.WriteString("<required> ")
	}
	if arg.Hint != nil {
		sb.WriteString(arg.Hint.String() + " ")
	}
	if arg.Variadic {
		sb.WriteString("...")
	}
	if arg.Ref {
		sb.WriteString("&")
	}
	sb.WriteString(fmt.Sprintf("$%s", arg.VarName))
	if !arg.Required {
		sb.WriteString(formatParamDefault(ctx, arg))
	}
	sb.WriteString(" ]\n")
	return sb.String()
}

// rcFormatProperty formats a non-static property for ReflectionClass::__toString().
// Output format: "    Property [ [abstract] public [protected(set)] [virtual] [readonly] [type] $name [= default] ]\n"
func rcFormatProperty(ctx phpv.Context, prop *phpv.ZClassProp, isReadonlyClass bool, isInterface bool) string {
	var sb strings.Builder
	sb.WriteString("    Property [ ")
	// Abstract modifier: explicit abstract on class property, or interface property (implicitly abstract)
	isAbstract := prop.Modifiers.Has(phpv.ZAttrAbstract) || (isInterface && prop.HasHooks)
	if isAbstract {
		sb.WriteString("abstract ")
	}
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
	} else if isReadonlyClass && prop.Modifiers.Has(phpv.ZAttrReadonly) && prop.SetModifiers == 0 {
		// In a readonly class, readonly properties implicitly have protected(set) visibility.
		// PHP 8.4 shows this explicitly in reflection output.
		sb.WriteString(" protected(set)")
	}
	// Virtual modifier: property has hooks but no backing store
	if prop.IsVirtual() {
		sb.WriteString(" virtual")
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
		if prop.GetHook != nil || prop.GetIsAbstract {
			if prop.GetIsFinal {
				sb.WriteString(" final get;")
			} else {
				sb.WriteString(" get;")
			}
		}
		if prop.SetHook != nil || prop.SetIsAbstract {
			if prop.SetIsFinal {
				sb.WriteString(" final set;")
			} else {
				sb.WriteString(" set;")
			}
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

func reflectionMethodHasTentativeReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.method.TentativeReturnType && data.method.ReturnType != nil).ZVal(), nil
}

func reflectionMethodGetTentativeReturnType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getMethodData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if data.method.TentativeReturnType && data.method.ReturnType != nil {
		return createReflectionTypeObject(ctx, data.method.ReturnType)
	}
	return phpv.ZNULL.ZVal(), nil
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
	// Support late static binding: if called on a subclass, create that subclass's instance
	targetClass := phpv.ZClass(ReflectionMethod)
	if cc, ok2 := ctx.(interface{ CalledClass() phpv.ZClass }); ok2 {
		if called := cc.CalledClass(); called != nil && called != ReflectionMethod {
			if zc, ok3 := called.(*phpobj.ZClass); ok3 && zc.InstanceOf(ReflectionMethod) {
				targetClass = zc
			}
		}
	}
	if targetClass == ReflectionMethod {
		return createReflectionMethodObject(ctx, class, method)
	}
	// Create an instance of the LSB class and initialize it
	obj, err2 := phpobj.CreateZObject(ctx, targetClass)
	if err2 != nil {
		return nil, err2
	}
	data := &reflectionMethodData{method: method, class: class}
	declaringClassName := class.GetName()
	if method.Class != nil {
		declaringClassName = method.Class.GetName()
	}
	obj.HashTable().SetString("name", method.Name.ZVal())
	obj.HashTable().SetString("class", declaringClassName.ZVal())
	obj.SetOpaque(ReflectionMethod, data)
	return obj.ZVal(), nil
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

// propertyHasRealDefault returns true if the property has an explicit default value.
// PHP distinguishes between:
// - Untyped properties with no default: treated as having null default (hasDefaultValue = true)
// - Typed properties with no default: truly no default (hasDefaultValue = false)
// - Dynamic properties: no default (hasDefaultValue = false)
func propertyHasRealDefault(data *reflectionPropertyData) bool {
	if data.isDynamic {
		return false
	}
	if data.prop.Default != nil {
		return true
	}
	// Untyped properties without explicit default have an implicit null default
	if data.prop.TypeHint == nil {
		return true
	}
	// Typed properties without explicit default have no default
	return false
}

func reflectionPropertyHasDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(propertyHasRealDefault(data)).ZVal(), nil
}

func reflectionPropertyGetDefaultValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getPropData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if !propertyHasRealDefault(data) {
		// PHP 8.5: return NULL with a deprecation notice when there's no real default
		_ = ctx.Deprecated("ReflectionProperty::getDefaultValue() for a property without a default value is deprecated, use ReflectionProperty::hasDefaultValue() to check if the default value exists", logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}
	if data.prop.Default == nil {
		// Untyped property with no explicit default - return null (no deprecation)
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
	// Variadic parameters are always optional in PHP reflection output
	if !data.arg.Required || data.arg.Variadic {
		sb.WriteString("<optional> ")
	} else {
		sb.WriteString("<required> ")
	}
	if data.arg.Hint != nil {
		sb.WriteString(data.arg.Hint.String() + " ")
	}
	if data.arg.Variadic {
		sb.WriteString("...")
	}
	sb.WriteString(fmt.Sprintf("$%s", data.arg.VarName))
	if data.arg.DefaultValue != nil {
		sb.WriteString(formatParamDefault(ctx, data.arg))
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

	// If we have a directly-captured callable (e.g., a closure), use it.
	if data.callable != nil {
		fData := &reflectionFunctionData{
			name:     data.funcName,
			callable: data.callable,
		}
		if fga, ok := data.callable.(phpv.FuncGetArgs); ok {
			fData.args = fga.GetArgs()
		}
		// Only set closure field for actual anonymous closures (not named functions).
		// Anonymous closures have names starting with "{closure".
		if zcl, ok := data.callable.(phpv.ZClosure); ok {
			name := zcl.Name()
			if strings.HasPrefix(name, "{closure") {
				fData.closure = zcl
			}
		}
		rfObj.SetOpaque(ReflectionFunction, fData)
	} else {
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
		} else {
			// Even if the function is not found in the registry (e.g., an anonymous closure
			// whose name was stored as funcName), create minimal function data so getName() works.
			fData := &reflectionFunctionData{
				name: data.funcName,
			}
			rfObj.SetOpaque(ReflectionFunction, fData)
		}
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
	data := getFuncData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	type extGetter interface {
		GetExt() string
	}
	if eg, ok := data.callable.(extGetter); ok {
		extName := eg.GetExt()
		if extName != "" {
			return phpv.ZString(extName).ZVal(), nil
		}
	}
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
	if data == nil || data.closure == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	// A "wrapped" closure (from first-class callable syntax like strlen(...) or
	// Closure::fromCallable('strlen')) is NOT anonymous - it wraps a named function.
	// Anonymous closures are those created with function() {} syntax.
	type wrappedChecker interface {
		IsWrapped() bool
	}
	if wc, ok := data.closure.(wrappedChecker); ok && wc.IsWrapped() {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
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
	data := getFuncData(o)
	arr := phpv.NewZArray()
	if data == nil || data.callable == nil {
		return arr.ZVal(), nil
	}
	if svg, ok := data.callable.(phpv.StaticVarGetter); ok {
		for _, entry := range svg.GetStaticVars(ctx) {
			arr.OffsetSet(ctx, entry.Name.ZVal(), entry.Val)
		}
	}
	return arr.ZVal(), nil
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
	extName := reflectionFunctionExtName(o)
	if extName == "" {
		return phpv.ZNULL.ZVal(), nil
	}
	extObj, err := phpobj.CreateZObject(ctx, ReflectionExtension)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	extObj.HashTable().SetString("name", phpv.ZString(extName).ZVal())
	extObj.SetOpaque(ReflectionExtension, phpv.ZString(extName))
	return extObj.ZVal(), nil
}

// reflectionFunctionExtName returns the extension name for a function's callable,
// or "" if it's user-defined or extension is unknown.
func reflectionFunctionExtName(o *phpobj.ZObject) string {
	data := getFuncData(o)
	if data == nil || data.callable == nil {
		return ""
	}
	type extGetter interface {
		GetExt() string
	}
	if eg, ok := data.callable.(extGetter); ok {
		return eg.GetExt()
	}
	return ""
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
		ReturnsByRef() bool
	}
	if rg, ok := data.callable.(refGetter); ok {
		return phpv.ZBool(rg.ReturnsByRef()).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

func reflectionFunctionToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getFuncData(o)
	if data == nil {
		return phpv.ZString("Function [ ]").ZVal(), nil
	}

	var sb strings.Builder

	// Determine if internal or user-defined
	type locGetter interface {
		Loc() *phpv.Loc
	}
	var funcLoc *phpv.Loc
	isInternal := true
	if lg, ok := data.callable.(locGetter); ok {
		funcLoc = lg.Loc()
		if funcLoc != nil {
			isInternal = false
		}
	}

	origin := "<user>"
	if isInternal {
		origin = "<internal>"
	}

	if data.closure != nil {
		sb.WriteString(fmt.Sprintf("Closure [ %s closure %s ] {\n", origin, data.name))
	} else {
		sb.WriteString(fmt.Sprintf("Function [ %s function %s ] {\n", origin, data.name))
	}

	// Source location (only for user-defined functions)
	if funcLoc != nil {
		type locEndGetter interface {
			LocEnd() *phpv.Loc
		}
		endLine := funcLoc.Line
		if leg, ok := data.callable.(locEndGetter); ok {
			if locEnd := leg.LocEnd(); locEnd != nil {
				endLine = locEnd.Line
			}
		}
		sb.WriteString(fmt.Sprintf("  @@ %s %d - %d\n", funcLoc.Filename, funcLoc.Line, endLine))
	}

	if data.args != nil && len(data.args) > 0 {
		sb.WriteString(fmt.Sprintf("\n  - Parameters [%d] {\n", len(data.args)))
		for i, arg := range data.args {
			sb.WriteString(rcFormatParameter(ctx, i, arg, "    "))
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
	type locEndGetter interface {
		LocEnd() *phpv.Loc
	}
	if leg, ok := data.callable.(locEndGetter); ok {
		loc := leg.LocEnd()
		if loc != nil {
			return phpv.ZInt(loc.Line).ZVal(), nil
		}
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
	data := getConstData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	extName := phpctx.GetConstantExtName(string(data.name))
	if extName == "" {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZString(extName).ZVal(), nil
}

func reflectionConstantGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getConstData(o)
	if data == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	extName := phpctx.GetConstantExtName(string(data.name))
	if extName == "" {
		return phpv.ZNULL.ZVal(), nil
	}
	extObj, err := phpobj.CreateZObject(ctx, ReflectionExtension)
	if err != nil {
		return phpv.ZNULL.ZVal(), nil
	}
	extObj.HashTable().SetString("name", phpv.ZString(extName).ZVal())
	extObj.SetOpaque(ReflectionExtension, phpv.ZString(extName))
	return extObj.ZVal(), nil
}

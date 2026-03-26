package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Lazy object option flags (as defined in PHP 8.4)
const (
	LazyObjectSkipInitOnSerialize = 8
	LazyObjectSkipDestructor      = 16
)

// Valid option masks
const (
	lazyNewGhostValidMask  = LazyObjectSkipInitOnSerialize
	lazyNewProxyValidMask  = LazyObjectSkipInitOnSerialize
	lazyResetValidMask     = LazyObjectSkipInitOnSerialize | LazyObjectSkipDestructor
)

// isDeclaredProp checks if a property is declared (not dynamic) in the class hierarchy.
func isDeclaredProp(class *phpobj.ZClass, name phpv.ZString) bool {
	for cur := class; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.VarName == name {
				return true
			}
		}
	}
	return false
}

// checkNoInternalClass checks if the class or any parent class is internal (defined in Go).
// Returns an error if so, nil otherwise.
func checkNoInternalClass(ctx phpv.Context, class phpv.ZClass) error {
	zc, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil
	}
	// Check the class itself
	if zc.L == nil {
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot make instance of internal class lazy: %s is internal", class.GetName()))
	}
	// Check parent classes
	for cur := zc.Extends; cur != nil; cur = cur.Extends {
		if cur.L == nil {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot make instance of internal class lazy: %s inherits internal class %s",
					class.GetName(), cur.GetName()))
		}
	}
	return nil
}

func initLazyObjectMethods() {
	// Add lazy object methods to ReflectionClass
	ReflectionClass.Methods["newlazyghost"] = &phpv.ZClassMethod{
		Name:   "newLazyGhost",
		Method: phpobj.NativeMethod(reflectionClassNewLazyGhost),
	}
	ReflectionClass.Methods["newlazyproxy"] = &phpv.ZClassMethod{
		Name:   "newLazyProxy",
		Method: phpobj.NativeMethod(reflectionClassNewLazyProxy),
	}
	ReflectionClass.Methods["isuninitializedlazyobject"] = &phpv.ZClassMethod{
		Name:   "isUninitializedLazyObject",
		Method: phpobj.NativeMethod(reflectionClassIsUninitializedLazyObject),
	}
	ReflectionClass.Methods["initializelazyobject"] = &phpv.ZClassMethod{
		Name:   "initializeLazyObject",
		Method: phpobj.NativeMethod(reflectionClassInitializeLazyObject),
	}
	ReflectionClass.Methods["marklazyobjectasinitialized"] = &phpv.ZClassMethod{
		Name:   "markLazyObjectAsInitialized",
		Method: phpobj.NativeMethod(reflectionClassMarkLazyObjectAsInitialized),
	}
	ReflectionClass.Methods["resetaslazyghost"] = &phpv.ZClassMethod{
		Name:   "resetAsLazyGhost",
		Method: phpobj.NativeMethod(reflectionClassResetAsLazyGhost),
	}
	ReflectionClass.Methods["resetaslazyproxy"] = &phpv.ZClassMethod{
		Name:   "resetAsLazyProxy",
		Method: phpobj.NativeMethod(reflectionClassResetAsLazyProxy),
	}
	ReflectionClass.Methods["getlazyinitializer"] = &phpv.ZClassMethod{
		Name:   "getLazyInitializer",
		Method: phpobj.NativeMethod(reflectionClassGetLazyInitializer),
	}

	// Add lazy object methods to ReflectionProperty
	ReflectionProperty.Methods["skiplazyinitialization"] = &phpv.ZClassMethod{
		Name:   "skipLazyInitialization",
		Method: phpobj.NativeMethod(reflectionPropertySkipLazyInitialization),
	}
	ReflectionProperty.Methods["setrawvaluewithoutlazyinitialization"] = &phpv.ZClassMethod{
		Name:   "setRawValueWithoutLazyInitialization",
		Method: phpobj.NativeMethod(reflectionPropertySetRawValueWithoutLazyInitialization),
	}
	ReflectionProperty.Methods["islazy"] = &phpv.ZClassMethod{
		Name:   "isLazy",
		Method: phpobj.NativeMethod(reflectionPropertyIsLazy),
	}
}

// reflectionClassNewLazyGhost creates a new lazy ghost object.
// Signature: public ReflectionClass::newLazyGhost(callable $initializer, int $options = 0): object
func reflectionClassNewLazyGhost(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::newLazyGhost() expects at least 1 argument, 0 given")
	}

	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Validate options
	options := int64(0)
	if len(args) > 1 && args[1].GetType() != phpv.ZtNull {
		options = int64(args[1].AsInt(ctx))
	}
	if options&^int64(lazyNewGhostValidMask) != 0 {
		// Check if there are completely invalid bits (outside all known flags)
		if options&^int64(lazyResetValidMask) != 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				"ReflectionClass::newLazyGhost(): Argument #2 ($options) contains invalid flags")
		}
		if options&int64(LazyObjectSkipDestructor) != 0 {
			return nil, phpobj.ThrowError(ctx, ReflectionException,
				"ReflectionClass::newLazyGhost(): Argument #2 ($options) does not accept ReflectionClass::SKIP_DESTRUCTOR")
		}
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			"ReflectionClass::newLazyGhost(): Argument #2 ($options) contains invalid flags")
	}

	// Check for internal classes (classes defined in Go have no source location)
	if err := checkNoInternalClass(ctx, class); err != nil {
		return nil, err
	}

	// Create the object without calling constructor
	obj, err := phpobj.CreateZObject(ctx, class)
	if err != nil {
		return nil, err
	}

	// Set up as lazy ghost
	obj.MakeLazyGhost(args[0])

	return obj.ZVal(), nil
}

// reflectionClassNewLazyProxy creates a new lazy proxy object.
// Signature: public ReflectionClass::newLazyProxy(callable $factory, int $options = 0): object
func reflectionClassNewLazyProxy(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::newLazyProxy() expects at least 1 argument, 0 given")
	}

	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Validate options
	options := int64(0)
	if len(args) > 1 && args[1].GetType() != phpv.ZtNull {
		options = int64(args[1].AsInt(ctx))
	}
	if options&^int64(lazyNewProxyValidMask) != 0 {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			"ReflectionClass::newLazyProxy(): Argument #2 ($options) contains invalid flags")
	}

	// Check for internal classes (classes defined in Go have no source location)
	if err := checkNoInternalClass(ctx, class); err != nil {
		return nil, err
	}

	// Create the object without calling constructor
	obj, err := phpobj.CreateZObject(ctx, class)
	if err != nil {
		return nil, err
	}

	// Set up as lazy proxy
	obj.MakeLazyProxy(args[0])

	return obj.ZVal(), nil
}

// reflectionClassIsUninitializedLazyObject checks if an object is an uninitialized lazy object.
// Signature: public ReflectionClass::isUninitializedLazyObject(object $object): bool
func reflectionClassIsUninitializedLazyObject(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::isUninitializedLazyObject() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::isUninitializedLazyObject(): Argument #1 ($object) must be of type object")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(obj.IsLazy()).ZVal(), nil
}

// reflectionClassInitializeLazyObject forces initialization of a lazy object.
// Signature: public ReflectionClass::initializeLazyObject(object $object): object
func reflectionClassInitializeLazyObject(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::initializeLazyObject() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::initializeLazyObject(): Argument #1 ($object) must be of type object")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return args[0], nil
	}

	if obj.IsLazy() {
		if err := obj.TriggerLazyInit(ctx); err != nil {
			return nil, err
		}
	}

	// For initialized proxies, return the real instance
	if obj.LazyState == phpobj.LazyProxyInitialized && obj.LazyInstance != nil {
		return obj.LazyInstance.ZVal(), nil
	}

	return args[0], nil
}

// reflectionClassMarkLazyObjectAsInitialized marks a lazy object as initialized
// without calling its initializer. Properties are set to their default values.
// Signature: public ReflectionClass::markLazyObjectAsInitialized(object $object): object
func reflectionClassMarkLazyObjectAsInitialized(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::markLazyObjectAsInitialized() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::markLazyObjectAsInitialized(): Argument #1 ($object) must be of type object")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return args[0], nil
	}

	obj.MarkLazyAsInitialized(ctx)

	return args[0], nil
}

// reflectionClassResetAsLazyGhost resets an existing object to be a lazy ghost.
// Signature: public ReflectionClass::resetAsLazyGhost(object $object, callable $initializer, int $options = 0): void
func reflectionClassResetAsLazyGhost(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::resetAsLazyGhost() expects at least 2 arguments")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::resetAsLazyGhost(): Argument #1 ($object) must be of type object")
	}

	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Validate options
	options := int64(0)
	if len(args) > 2 && args[2].GetType() != phpv.ZtNull {
		options = int64(args[2].AsInt(ctx))
	}
	if options&^int64(lazyResetValidMask) != 0 {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			"ReflectionClass::resetAsLazyGhost(): Argument #3 ($options) contains invalid flags")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid object")
	}

	// Validate class compatibility: the object must be an instance of this class
	if !obj.Class.InstanceOf(class) && obj.Class.GetName() != class.GetName() {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("ReflectionClass::resetAsLazyGhost(): Argument #1 ($object) must be of type %s, %s given",
				class.GetName(), obj.Class.GetName()))
	}

	// Cannot reset an already lazy object
	if obj.IsLazy() {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Object is already lazy")
	}

	// Call destructor if not skipped
	if options&int64(LazyObjectSkipDestructor) == 0 {
		_ = obj.CallImplicitDestructor(ctx)
	}

	// Reset destructed flag
	obj.SetDestructed(false)

	// Reset as lazy ghost
	obj.MakeLazyGhost(args[1])

	return nil, nil
}

// reflectionClassResetAsLazyProxy resets an existing object to be a lazy proxy.
// Signature: public ReflectionClass::resetAsLazyProxy(object $object, callable $factory, int $options = 0): void
func reflectionClassResetAsLazyProxy(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::resetAsLazyProxy() expects at least 2 arguments")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::resetAsLazyProxy(): Argument #1 ($object) must be of type object")
	}

	class := getClassData(o)
	if class == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object")
	}

	// Validate options
	options := int64(0)
	if len(args) > 2 && args[2].GetType() != phpv.ZtNull {
		options = int64(args[2].AsInt(ctx))
	}
	if options&^int64(lazyResetValidMask) != 0 {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			"ReflectionClass::resetAsLazyProxy(): Argument #3 ($options) contains invalid flags")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Invalid object")
	}

	// Validate class compatibility
	if !obj.Class.InstanceOf(class) && obj.Class.GetName() != class.GetName() {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("ReflectionClass::resetAsLazyProxy(): Argument #1 ($object) must be of type %s, %s given",
				class.GetName(), obj.Class.GetName()))
	}

	// Cannot reset an already lazy object (but initialized proxies can be re-reset)
	if obj.IsLazy() {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Object is already lazy")
	}

	// Call destructor if not skipped
	if options&int64(LazyObjectSkipDestructor) == 0 {
		_ = obj.CallImplicitDestructor(ctx)
	}

	// Reset destructed flag
	obj.SetDestructed(false)

	// Reset as lazy proxy
	obj.MakeLazyProxy(args[1])

	return nil, nil
}

// reflectionClassGetLazyInitializer returns the initializer/factory of a lazy object, or null.
// Signature: public ReflectionClass::getLazyInitializer(object $object): ?callable
func reflectionClassGetLazyInitializer(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionClass::getLazyInitializer() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionClass::getLazyInitializer(): Argument #1 ($object) must be of type object")
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}

	if obj.LazyInitializer != nil {
		return obj.LazyInitializer, nil
	}

	return phpv.ZNULL.ZVal(), nil
}

// reflectionPropertySkipLazyInitialization marks a property as skipped for lazy initialization.
// Signature: public ReflectionProperty::skipLazyInitialization(object $object): void
func reflectionPropertySkipLazyInitialization(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionProperty::skipLazyInitialization() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionProperty::skipLazyInitialization(): Argument #1 ($object) must be of type object")
	}

	data := o.GetOpaque(ReflectionProperty)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve property data")
	}
	propData, ok := data.(*reflectionPropertyData)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve property data")
	}

	prop := propData.prop
	propClass := propData.class

	// Cannot skip static properties
	if prop.Modifiers.IsStatic() {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use skipLazyInitialization on static property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	// Cannot skip virtual properties
	if prop.IsVirtual() {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use skipLazyInitialization on virtual property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	// Cannot use on dynamic properties
	if propData.class == nil || !isDeclaredProp(propData.class, prop.VarName) {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use skipLazyInitialization on dynamic property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil
	}

	if obj.IsLazy() {
		obj.SkipLazyProperty(ctx, prop.VarName)
	}

	return nil, nil
}

// reflectionPropertySetRawValueWithoutLazyInitialization sets a property value
// without triggering lazy initialization.
// Signature: public ReflectionProperty::setRawValueWithoutLazyInitialization(object $object, mixed $value): void
func reflectionPropertySetRawValueWithoutLazyInitialization(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionProperty::setRawValueWithoutLazyInitialization() expects exactly 2 arguments")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionProperty::setRawValueWithoutLazyInitialization(): Argument #1 ($object) must be of type object")
	}

	data := o.GetOpaque(ReflectionProperty)
	if data == nil {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve property data")
	}
	propData, ok := data.(*reflectionPropertyData)
	if !ok {
		return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve property data")
	}

	prop := propData.prop
	propClass := propData.class

	// Cannot use on static properties
	if prop.Modifiers.IsStatic() {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use setRawValueWithoutLazyInitialization on static property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	// Cannot use on virtual properties
	if prop.IsVirtual() {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use setRawValueWithoutLazyInitialization on virtual property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	// Cannot use on dynamic properties (check if this is a declared property)
	if propData.class == nil || !isDeclaredProp(propData.class, prop.VarName) {
		return nil, phpobj.ThrowError(ctx, ReflectionException,
			fmt.Sprintf("Can not use setRawValueWithoutLazyInitialization on dynamic property %s::$%s",
				propClass.GetName(), prop.VarName))
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil
	}

	if obj.IsLazy() {
		obj.SetRawValueWithoutLazyInit(ctx, prop.VarName, args[1])
	} else {
		// For non-lazy objects, just set the value directly bypassing hooks
		if prop.Modifiers.IsPrivate() {
			k := phpobj.GetPrivatePropNameExt(propClass, prop.VarName)
			obj.HashTable().SetString(k, args[1])
		} else {
			obj.HashTable().SetString(prop.VarName, args[1])
		}
	}

	return nil, nil
}

// reflectionPropertyIsLazy checks if a property is lazy (not yet initialized) on the given object.
// Signature: public ReflectionProperty::isLazy(object $object): bool
func reflectionPropertyIsLazy(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"ReflectionProperty::isLazy() expects exactly 1 argument, 0 given")
	}

	if args[0].GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"ReflectionProperty::isLazy(): Argument #1 ($object) must be of type object")
	}

	data := o.GetOpaque(ReflectionProperty)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	propData, ok := data.(*reflectionPropertyData)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	prop := propData.prop

	// Static properties are never lazy
	if prop.Modifiers.IsStatic() {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Virtual properties are never lazy
	if prop.IsVirtual() {
		return phpv.ZBool(false).ZVal(), nil
	}

	obj, ok := args[0].Value().(*phpobj.ZObject)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	// If the object is not lazy at all, properties are not lazy
	if !obj.IsLazy() && obj.LazyState != phpobj.LazyProxyInitialized {
		return phpv.ZBool(false).ZVal(), nil
	}

	// For initialized proxies, check if the real instance has lazy props
	if obj.LazyState == phpobj.LazyProxyInitialized && obj.LazyInstance != nil {
		target := obj.ResolveProxy()
		if target.IsLazy() {
			// The real instance is itself lazy - properties that are not skipped are lazy
			if target.IsPropertySkippedForLazy(prop.VarName) {
				return phpv.ZBool(false).ZVal(), nil
			}
			return phpv.ZBool(true).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// For uninitialized lazy objects, a property is lazy unless it's been skipped
	if obj.IsLazy() {
		if obj.IsPropertySkippedForLazy(prop.VarName) {
			return phpv.ZBool(false).ZVal(), nil
		}
		return phpv.ZBool(true).ZVal(), nil
	}

	return phpv.ZBool(false).ZVal(), nil
}

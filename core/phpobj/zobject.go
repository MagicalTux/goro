package phpobj

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"sync/atomic"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZObject struct {
	h          *phpv.ZHashTable
	hasPrivate map[phpv.ZString]struct{}

	Class        phpv.ZClass
	CurrentClass phpv.ZClass

	// for use with custom extension objects
	Opaque map[phpv.ZClass]interface{}
	ID     int

	// Guards for __get/__set/__isset/__unset to prevent infinite recursion
	getGuard   map[phpv.ZString]bool
	setGuard   map[phpv.ZString]bool
	issetGuard map[phpv.ZString]bool
	unsetGuard map[phpv.ZString]bool

	// Guards for property hook execution to prevent infinite recursion
	// When a get/set hook accesses $this->propName for the same property,
	// the guard ensures the backing value is accessed directly.
	getHookGuard map[phpv.ZString]bool
	setHookGuard map[phpv.ZString]bool

	// Readonly property tracking - set of properties that have been initialized
	readonlyInit map[phpv.ZString]bool

	// Tracks typed properties that were explicitly unset (to distinguish from
	// "never initialized"). Unset typed properties allow __set/__unset fallback,
	// while never-initialized typed properties throw the visibility error directly.
	// Stored as a pointer so wrapper objects share the same map.
	typedPropUnset *map[phpv.ZString]bool

	// Destructor tracking - stored as a pointer so wrapper objects share the flag.
	destructed *bool

	// Reference counting for destructor timing.
	// Stored as a pointer so that wrapper objects (from GetKin/Unwrap/new)
	// share the same refcount with the original.
	refCount *int32

	// jsonApplyCount tracks recursive json_encode depth.
	// Mirrors PHP's GC_PROTECT_RECURSION mechanism.
	// Stored as a pointer so wrapper objects share the same counter.
	jsonApplyCount *int32

	// serializeApplyCount tracks recursive serialize depth.
	// Stored as a pointer so wrapper objects share the same counter.
	serializeApplyCount *int32

	// Lazy object support (PHP 8.4) - these fields are propagated to wrapper objects
	// by the new() method so that wrappers share lazy state with the original.
	// LazyState tracks whether this is a lazy ghost/proxy and its initialization state.
	// 0 = not lazy, 1 = lazy ghost (uninitialized), 2 = lazy proxy (uninitialized),
	// 3 = lazy ghost (initialized), 4 = lazy proxy (initialized)
	LazyState int

	// LazyInitializer is the callback for ghost objects or the factory for proxy objects.
	LazyInitializer *phpv.ZVal

	// LazyInstance holds the real object for initialized proxy objects.
	LazyInstance *ZObject

	// LazySkippedProps tracks properties that have been skipped via
	// ReflectionProperty::skipLazyInitialization() - accessing these does not trigger init.
	LazySkippedProps map[phpv.ZString]bool

	// LazyInitializing is true while the initializer/factory is being called,
	// to prevent recursive initialization.
	LazyInitializing bool
}

// CallDestructor calls __destruct on this object if it hasn't been called yet.
// It checks visibility of the destructor against the calling context.
func (z *ZObject) CallDestructor(ctx phpv.Context) error {
	if z.IsDestructed() {
		return nil
	}
	// Lazy objects that were never initialized should not have their destructor called
	if z.IsLazy() {
		return nil
	}
	m, ok := z.Class.GetMethod("__destruct")
	if !ok {
		return nil
	}
	z.SetDestructed(true)
	// Unregister from shutdown destructor list
	ctx.Global().UnregisterDestructor(z)

	// Check visibility
	if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if m.Modifiers.IsPrivate() {
			if callerClass == nil || callerClass.GetName() != z.Class.GetName() {
				vis := "private"
				scope := "global scope"
				if callerClass != nil {
					scope = fmt.Sprintf("scope %s", callerClass.GetName())
				}
				return ThrowError(ctx, Error,
					fmt.Sprintf("Call to %s %s::__destruct() from %s",
						vis, z.Class.GetName(), scope))
			}
		} else { // protected
			if callerClass == nil || (!callerClass.InstanceOf(z.Class) && !z.Class.InstanceOf(callerClass)) {
				vis := "protected"
				scope := "global scope"
				if callerClass != nil {
					scope = fmt.Sprintf("scope %s", callerClass.GetName())
				}
				return ThrowError(ctx, Error,
					fmt.Sprintf("Call to %s %s::__destruct() from %s",
						vis, z.Class.GetName(), scope))
			}
		}
	}

	_, err := ctx.CallZVal(ctx, m.Method, nil, z)
	return err
}


// CallImplicitDestructor calls __destruct without visibility checks.
// Used for implicit destruction (variable overwrite, shutdown) where
// PHP always allows the destructor to run regardless of visibility.
func (z *ZObject) CallImplicitDestructor(ctx phpv.Context) error {
	if z.IsDestructed() {
		return nil
	}
	// Lazy objects that were never initialized should not have their destructor called
	if z.IsLazy() {
		return nil
	}
	m, ok := z.Class.GetMethod("__destruct")
	if !ok {
		return nil
	}
	z.SetDestructed(true)
	ctx.Global().UnregisterDestructor(z)
	_, err := ctx.CallZVal(ctx, m.Method, nil, z)
	return err
}

// IncRef increments the object's reference count.
func (z *ZObject) IncRef() {
	if z.refCount == nil {
		z.refCount = new(int32)
	}
	atomic.AddInt32(z.refCount, 1)
}

// DecRef decrements the object's reference count and calls the destructor
// (with visibility checks) when the count reaches zero.
func (z *ZObject) DecRef(ctx phpv.Context) error {
	if z.refCount == nil {
		z.refCount = new(int32)
	}
	n := atomic.AddInt32(z.refCount, -1)
	if n <= 0 {
		return z.CallDestructor(ctx)
	}
	return nil
}

// DecRefImplicit decrements the object's reference count and calls the
// destructor without visibility checks when the count reaches zero.
// Used for scope exit where PHP always allows destructors to run.
func (z *ZObject) DecRefImplicit(ctx phpv.Context) error {
	if z.refCount == nil {
		z.refCount = new(int32)
	}
	n := atomic.AddInt32(z.refCount, -1)
	if n <= 0 {
		return z.CallImplicitDestructor(ctx)
	}
	return nil
}

// RefCount returns the current reference count.
func (z *ZObject) RefCount() int32 {
	if z.refCount == nil {
		return 0
	}
	return atomic.LoadInt32(z.refCount)
}

// IsDestructed returns whether the destructor has already been called.
func (z *ZObject) IsDestructed() bool {
	if z.destructed == nil {
		return false
	}
	return *z.destructed
}

// SetDestructed sets the destructed flag.
func (z *ZObject) SetDestructed(v bool) {
	if z.destructed == nil {
		z.destructed = new(bool)
	}
	*z.destructed = v
}

// setPropRefTracked sets a property on the hash table while tracking object
// reference counts. When a new object is stored, IncRef is called. When an old
// object value is replaced or removed (value == nil), the old object's refcount
// is not decremented here -- callers can do that separately if needed.
// This ensures that objects stored in properties don't get their destructors
// fired prematurely when the source variable goes out of scope.
func (z *ZObject) setPropRefTracked(key phpv.ZString, value *phpv.ZVal) error {
	// IncRef the new value if it's an object
	if value != nil && value.GetType() == phpv.ZtObject {
		if obj, ok := value.Value().(*ZObject); ok {
			obj.IncRef()
		}
	}
	return z.h.SetString(key, value)
}

// IncrJsonApplyCount increments the json_encode recursion guard counter.
// Returns the count BEFORE incrementing. If > 0, the object is already being json-encoded.
func (z *ZObject) IncrJsonApplyCount() int32 {
	if z.jsonApplyCount == nil {
		z.jsonApplyCount = new(int32)
	}
	old := *z.jsonApplyCount
	*z.jsonApplyCount++
	return old
}

// DecrJsonApplyCount decrements the json_encode recursion guard counter.
func (z *ZObject) DecrJsonApplyCount() {
	if z.jsonApplyCount != nil && *z.jsonApplyCount > 0 {
		*z.jsonApplyCount--
	}
}

// IncrSerializeApplyCount increments the serialize recursion guard counter.
// Returns the count BEFORE incrementing. If > 0, the object is already being serialized.
func (z *ZObject) IncrSerializeApplyCount() int32 {
	if z.serializeApplyCount == nil {
		z.serializeApplyCount = new(int32)
	}
	old := *z.serializeApplyCount
	*z.serializeApplyCount++
	return old
}

// DecrSerializeApplyCount decrements the serialize recursion guard counter.
func (z *ZObject) DecrSerializeApplyCount() {
	if z.serializeApplyCount != nil && *z.serializeApplyCount > 0 {
		*z.serializeApplyCount--
	}
}

func (z *ZObject) ZVal() *phpv.ZVal {
	return phpv.MakeZVal(z)
}

func (z *ZObject) GetType() phpv.ZType {
	return phpv.ZtObject
}

func (z *ZObject) GetOpaque(c phpv.ZClass) interface{} {
	if z.Opaque == nil {
		return nil
	}
	v, ok := z.Opaque[c]
	if !ok {
		return nil
	}
	return v
}

// GetOpaqueByName looks up opaque data by class name instead of class pointer.
// This is useful to break initialization cycles where the class variable isn't
// available yet during init.
func (z *ZObject) GetOpaqueByName(name string) interface{} {
	if z.Opaque == nil {
		return nil
	}
	for c, v := range z.Opaque {
		if c.GetName() == phpv.ZString(name) {
			return v
		}
	}
	return nil
}

func (z *ZObject) SetOpaque(c phpv.ZClass, v interface{}) {
	if z.Opaque == nil {
		z.Opaque = make(map[phpv.ZClass]interface{})
	}
	z.Opaque[c] = v
}

func (z *ZObject) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	// Check for custom cast handler (e.g., GMP)
	if h := z.Class.Handlers(); h != nil && h.HandleCast != nil {
		if t == phpv.ZtInt || t == phpv.ZtFloat || t == phpv.ZtBool {
			return h.HandleCast(ctx, z, t)
		}
	}

	switch t {
	case phpv.ZtString:
		if m, ok := z.Class.GetMethod("__tostring"); ok {
			result, err := ctx.CallZVal(ctx, m.Method, nil, z)
			if err != nil {
				return nil, err
			}
			if result == nil || result.GetType() == phpv.ZtNull {
				return nil, ThrowError(ctx, Error, fmt.Sprintf("%s::__toString(): Return value must be of type string, none returned", z.Class.GetName()))
			}
			if result.GetType() != phpv.ZtString {
				return nil, ThrowError(ctx, Error, fmt.Sprintf("%s::__toString(): Return value must be of type string, %s returned", z.Class.GetName(), result.GetType()))
			}
			return result, nil
		}
		// String-backed enums can be implicitly cast to string (returning their backing value)
		if zc, ok := z.Class.(*ZClass); ok && zc.Type.Has(phpv.ZClassTypeEnum) && zc.EnumBackingType == phpv.ZtString {
			if backingVal := z.h.GetString("value"); backingVal != nil {
				return backingVal.Value().(phpv.ZString), nil
			}
		}
	case phpv.ZtInt:
		ctx.Warn("Object of class %s could not be converted to int", z.Class.GetName(), logopt.NoFuncName(true))
		return phpv.ZInt(1), nil
	case phpv.ZtBool:
		return phpv.ZBool(true), nil
	case phpv.ZtFloat:
		ctx.Warn("Object of class %s could not be converted to float", z.Class.GetName(), logopt.NoFuncName(true))
		return phpv.ZFloat(1), nil
	case phpv.ZtArray:
		// Closure objects cast to array as [0 => $closure], not property iteration
		if z.Class.GetName() == "Closure" {
			arr := phpv.NewZArray()
			arr.OffsetSet(ctx, nil, z.ZVal())
			return arr, nil
		}
		// Check for custom array cast handler (e.g., ArrayObject)
		if h := z.Class.Handlers(); h != nil && h.HandleCastArray != nil {
			return h.HandleCastArray(ctx, z)
		}
		return z.toArray(ctx), nil
	}

	if t == phpv.ZtString {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Object of class %s could not be converted to string", z.Class.GetName()))
	}
	return nil, ctx.Errorf("failed to convert object to %s", t)
}

// toArray converts an object to an array with PHP's property name mangling.
// For uninitialized lazy objects, returns only the currently-set properties
// (does NOT trigger initialization).
func (z *ZObject) toArray(ctx phpv.Context) *phpv.ZArray {
	// For uninitialized lazy objects, return only the hash table entries
	if z.IsLazy() {
		arr := phpv.NewZArray()
		it := z.h.NewIterator()
		for it.Valid(ctx) {
			k, _ := it.Key(ctx)
			v, _ := it.Current(ctx)
			arr.OffsetSet(ctx, k, v)
			it.Next(ctx)
		}
		return arr
	}
	// For initialized proxies, delegate to the real instance
	if z.LazyState == LazyProxyInitialized && z.LazyInstance != nil {
		return z.LazyInstance.toArray(ctx)
	}

	arr := phpv.NewZArray()
	for prop := range z.IterProps(ctx) {
		var key phpv.ZString
		if prop.Modifiers.IsPrivate() {
			className := string(z.GetDeclClassName(prop))
			key = phpv.ZString("\x00" + className + "\x00" + string(prop.VarName))
		} else if prop.Modifiers.IsProtected() {
			key = phpv.ZString("\x00*\x00" + string(prop.VarName))
		} else {
			key = prop.VarName
		}
		v := z.GetPropValue(prop)
		arr.OffsetSet(ctx, key, v)
	}
	return arr
}

// Similar to NewZObject, but without calling the constructor
func CreateZObject(ctx phpv.Context, c phpv.ZClass) (*ZObject, error) {
	if c == nil {
		c = StdClass
	}

	tpu := make(map[phpv.ZString]bool)
	n := &ZObject{
		h:              phpv.NewHashTable(),
		hasPrivate:     make(map[phpv.ZString]struct{}),
		Class:          c,
		ID:             ctx.Global().NextObjectID(),
		Opaque:         map[phpv.ZClass]interface{}{},
		typedPropUnset: &tpu,
		refCount:       new(int32),
		destructed:     new(bool),
	}

	err := n.init(ctx)
	if err != nil {
		return nil, err
	}

	// Register for destructor call at shutdown if __destruct exists
	if _, ok := c.GetMethod("__destruct"); ok {
		ctx.Global().RegisterDestructor(n)
	}

	return n, nil
}

// isExceptionOrError checks if a class is Exception, Error, or extends either.
// This walks only the Extends chain to avoid infinite recursion through interfaces.
func isExceptionOrError(c phpv.ZClass) bool {
	zc, ok := c.(*ZClass)
	if !ok {
		return false
	}
	for zc != nil {
		if zc == Exception || zc == Error {
			return true
		}
		zc = zc.Extends
	}
	return false
}

func NewZObject(ctx phpv.Context, c phpv.ZClass, args ...*phpv.ZVal) (*ZObject, error) {
	if c == nil {
		c = StdClass
	}

	// Check if class is an interface
	if zc, ok := c.(*ZClass); ok && zc.Type == phpv.ZClassTypeInterface {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Cannot instantiate interface %s", c.GetName()))
	}

	// Check if class is abstract
	if zc, ok := c.(*ZClass); ok && zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Cannot instantiate abstract class %s", c.GetName()))
	}

	// Check if class is an enum (enums cannot be instantiated with new)
	if c.GetType()&phpv.ZClassTypeEnum != 0 {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Cannot instantiate enum %s", c.GetName()))
	}

	// Check if class is a trait
	if c.GetType() == phpv.ZClassTypeTrait {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Cannot instantiate trait %s", c.GetName()))
	}

	// Check for classes that cannot be instantiated directly (e.g., Closure, Generator)
	if zc, ok := c.(*ZClass); ok && zc.InternalOnly {
		if zc == Generator {
			return nil, ThrowError(ctx, Error, fmt.Sprintf("The \"%s\" class is reserved for internal use and cannot be manually instantiated", c.GetName()))
		}
		if zc.H != nil && zc.H.HandleInvoke != nil {
			return nil, ThrowError(ctx, Error, fmt.Sprintf("Instantiation of class %s is not allowed", c.GetName()))
		}
	}

	tpu := make(map[phpv.ZString]bool)
	n := &ZObject{
		h:              phpv.NewHashTable(),
		hasPrivate:     make(map[phpv.ZString]struct{}),
		Class:          c,
		ID:             ctx.Global().NextObjectID(),
		Opaque:         map[phpv.ZClass]interface{}{},
		typedPropUnset: &tpu,
		refCount:       new(int32),
		destructed:     new(bool),
	}
	var constructor phpv.Callable

	err := n.init(ctx)
	if err != nil {
		return nil, err
	}

	// Track object memory allocation
	if mt := ctx.Global().MemMgrTracker(); mt != nil {
		propCount := int64(n.h.Count())
		mt.MemAlloc(256 + propCount*64)
	}

	// Pre-set file/line for Exception/Error subclasses.
	// PHP sets these during object creation (before the constructor runs),
	// so even if a subclass overrides __construct without calling parent,
	// file/line are still set to where "new" was called.
	if isExceptionOrError(c) {
		loc := ctx.Loc()
		if loc != nil {
			n.h.SetString("file", phpv.ZString(loc.Filename).ZVal())
			n.h.SetString("line", phpv.ZInt(loc.Line).ZVal())
		}
	}

	var ctorMethod *phpv.ZClassMethod
	if n.Class.Handlers() != nil && n.Class.Handlers().Constructor != nil {
		ctorMethod = n.Class.Handlers().Constructor
		constructor = ctorMethod.Method
	} else if m, ok := n.Class.GetMethod("__construct"); ok {
		ctorMethod = m
		constructor = m.Method
	}

	if constructor != nil {
		// Check constructor visibility before calling
		if ctorMethod != nil {
			if ctorMethod.Modifiers.Has(phpv.ZAttrPrivate) {
				callerClass := ctx.Class()
				ctorClass := ctorMethod.Class
				if callerClass == nil || ctorClass == nil || callerClass.GetName() != ctorClass.GetName() {
					scope := "global scope"
					if callerClass != nil {
						scope = fmt.Sprintf("scope %s", callerClass.GetName())
					}
					return nil, ThrowError(ctx, Error, fmt.Sprintf("Call to private %s::__construct() from %s", c.GetName(), scope))
				}
			} else if ctorMethod.Modifiers.Has(phpv.ZAttrProtected) {
				callerClass := ctx.Class()
				if callerClass == nil {
					return nil, ThrowError(ctx, Error, fmt.Sprintf("Call to protected %s::__construct() from global scope", c.GetName()))
				}
				if !callerClass.InstanceOf(ctorMethod.Class) && !ctorMethod.Class.InstanceOf(callerClass) {
					return nil, ThrowError(ctx, Error, fmt.Sprintf("Call to protected %s::__construct() from scope %s", c.GetName(), callerClass.GetName()))
				}
			}
		}

		// Note: #[\Deprecated] check for user constructors is handled by ZClosure.Call()
		// which fires when the constructor body is actually invoked.

		// Handle constructor property promotion: set promoted properties before calling body.
		// We bypass ObjectSet (which checks visibility) and set directly on the hash table,
		// since constructor promotion always has access to its own properties.
		if fga, ok := constructor.(phpv.FuncGetArgs); ok {
			fargs := fga.GetArgs()
			for i, arg := range fargs {
				if arg.Promotion == 0 {
					continue
				}
				var val *phpv.ZVal
				if i < len(args) {
					val = args[i]
				} else if arg.DefaultValue != nil {
					// Resolve default value for promoted property when argument not passed
					if cd, ok := arg.DefaultValue.(*phpv.CompileDelayed); ok {
						resolved, err := cd.Run(ctx)
						if err != nil {
							return nil, err
						}
						arg.DefaultValue = resolved.Value()
						val = resolved
					} else {
						val = arg.DefaultValue.ZVal()
					}
				}
				if val == nil {
					continue
				}

				propName := phpv.ZString(arg.VarName)
				// If this promoted property has hooks, use ObjectSet to invoke hooks
				if arg.PromotionHooks != nil && arg.PromotionHooks.HasHooks && arg.PromotionHooks.SetHook != nil {
					if err := n.ObjectSet(ctx, propName, val); err != nil {
						return nil, err
					}
				} else if arg.Promotion.IsPrivate() {
					mangledName := getPrivatePropName(c, propName)
					n.h.SetString(mangledName, val)
				} else {
					n.h.SetString(propName, val)
				}
				// Mark readonly properties as initialized
				isReadonly := arg.Promotion.IsReadonly()
				if !isReadonly {
					// Check if this is a readonly class (all properties implicitly readonly)
					if ca, ok := c.(*ZClass); ok {
						isReadonly = ca.Attr.Has(phpv.ZClassReadonly)
					}
				}
				if isReadonly {
					if n.readonlyInit == nil {
						n.readonlyInit = make(map[phpv.ZString]bool)
					}
					n.readonlyInit[propName] = true
				}
			}
		}

		// Wrap the constructor in a MethodCallable so that ctx.Class() returns
		// the declaring class (not the instantiated class). This is important for
		// private property access and PHP 8.4 asymmetric visibility.
		// Use BindClassLSB to set CalledClass to the instantiated class (c) for
		// late static binding support (get_called_class() returns the actual class).
		ctorCallable := phpv.Callable(constructor)
		if ctorMethod != nil && ctorMethod.Class != nil {
			if _, ok := constructor.(*phpv.MethodCallable); !ok {
				ctorCallable = phpv.BindClassLSB(constructor, ctorMethod.Class, c, false)
			}
		}
		_, err := ctx.CallZVal(ctx, ctorCallable, args, n)
		if err != nil {
			return nil, err
		}
	}

	// Register for destructor call at shutdown if __destruct exists
	if _, ok := c.GetMethod("__destruct"); ok {
		ctx.Global().RegisterDestructor(n)
	}

	return n, nil
}

func (z *ZObject) GetKin(className string) phpv.ZObject {
	class := z.Class.(*ZClass)
	for class != nil {
		if class.GetName() == phpv.ZString(className) {
			return z.new(class)
		}
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}
	return nil
}

func (z *ZObject) Unwrap() phpv.ZObject {
	if z == nil {
		return z
	}
	// If no CurrentClass is set, no wrapping is needed - return self to
	// preserve the same *ZObject pointer (and its refcount).
	if z.CurrentClass == nil {
		return z
	}
	// For lazy objects, return self to preserve the lazy state pointer.
	// Creating a wrapper would lose lazy state synchronization.
	if z.IsLazy() || z.LazyState != LazyNone {
		return z
	}
	return z.new(nil)
}

func (z *ZObject) GetParent() phpv.ZObject {
	if z == nil || z.Class == nil {
		return nil
	}
	class, ok := z.GetClass().(*ZClass)
	if !ok || class == nil {
		return nil
	}
	if z.CurrentClass != nil {
		if cc, ok := z.CurrentClass.(*ZClass); ok && cc != nil {
			class = cc
		}
	}
	parent := class.GetParent()
	if parent == nil {
		return nil
	}
	parentClass, ok := parent.(*ZClass)
	if !ok || parentClass == nil {
		return nil
	}
	return z.new(parentClass)
}

func (z *ZObject) new(class *ZClass) *ZObject {
	return &ZObject{
		h:                   z.h,
		hasPrivate:          z.hasPrivate,
		Class:               z.Class,
		CurrentClass:        class,
		Opaque:              z.Opaque,
		ID:                  z.ID,
		readonlyInit:        z.readonlyInit,
		typedPropUnset:      z.typedPropUnset,
		refCount:            z.refCount,
		destructed:          z.destructed,
		jsonApplyCount:      z.jsonApplyCount,
		serializeApplyCount: z.serializeApplyCount,
		// Propagate lazy state so wrapper objects share it with the original
		LazyState:        z.LazyState,
		LazyInitializer:  z.LazyInitializer,
		LazyInstance:     z.LazyInstance,
		LazySkippedProps: z.LazySkippedProps,
		LazyInitializing: z.LazyInitializing,
	}
}

func (z *ZObject) Clone(ctx phpv.Context) (phpv.ZObject, error) {
	// Lazy objects: clone triggers initialization
	if z.IsLazy() {
		if err := z.TriggerLazyInit(ctx); err != nil {
			return nil, err
		}
	}

	opaque := map[phpv.ZClass]any{}
	if len(z.Opaque) != 0 {
		for class, thing := range z.Opaque {
			if cloneable, ok := thing.(phpv.Cloneable); ok {
				thing = cloneable.Clone()
			}
			opaque[class] = thing
		}
	}

	// For initialized proxies, the clone preserves the proxy structure
	if z.LazyState == LazyProxyInitialized && z.LazyInstance != nil {
		// Clone the proxy shell
		n := &ZObject{
			Class:        z.Class,
			CurrentClass: z.CurrentClass,
			h:            z.h.Dup(),
			hasPrivate:   maps.Clone(z.hasPrivate),
			Opaque:       opaque,
			ID:           ctx.Global().NextObjectID(),
			refCount:     new(int32),
			LazyState:    LazyProxyInitialized,
		}
		// Clone the real instance
		clonedReal, err := z.LazyInstance.Clone(ctx)
		if err != nil {
			return nil, err
		}
		n.LazyInstance = clonedReal.(*ZObject)
		return n, nil
	}

	n := &ZObject{
		Class:        z.Class,
		CurrentClass: z.CurrentClass,
		h:            z.h.Dup(), // copy on write
		hasPrivate:   maps.Clone(z.hasPrivate),
		Opaque:       opaque,
		ID:           ctx.Global().NextObjectID(),
		refCount:     new(int32),
	}

	// Call __clone() on the new object if it exists
	if m, ok := n.Class.GetMethod("__clone"); ok {
		_, err := ctx.CallZVal(ctx, m.Method, nil, n)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

// NewZObjectEnum creates a bare ZObject for an enum case without calling init()
// or resolving constants. This avoids infinite recursion since enum cases are
// stored as class constants themselves.
func NewZObjectEnum(ctx phpv.Context, c phpv.ZClass) *ZObject {
	return &ZObject{
		h:          phpv.NewHashTable(),
		Class:      c,
		hasPrivate: make(map[phpv.ZString]struct{}),
		Opaque:     map[phpv.ZClass]interface{}{},
		ID:         ctx.Global().NextObjectID(),
		refCount:   new(int32),
		destructed: new(bool),
	}
}

func NewZObjectOpaque(ctx phpv.Context, c phpv.ZClass, v interface{}) (*ZObject, error) {
	n := &ZObject{
		h:          phpv.NewHashTable(),
		Class:      c,
		Opaque:     map[phpv.ZClass]interface{}{c: v},
		hasPrivate: make(map[phpv.ZString]struct{}),
		ID:         ctx.Global().NextObjectID(),
	}
	err := n.init(ctx)
	if err != nil {
		return nil, err
	}

	// Register for destructor call at shutdown if __destruct exists
	if _, ok := c.GetMethod("__destruct"); ok {
		ctx.Global().RegisterDestructor(n)
	}

	return n, nil
}

// dupDefault creates a per-instance copy of a default property value.
// For arrays, this creates a proper duplicate so that each object instance
// gets its own independent array instead of sharing the class-level default.
func dupDefault(v phpv.Val) *phpv.ZVal {
	if arr, ok := v.(*phpv.ZArray); ok {
		return arr.Dup().ZVal()
	}
	return v.ZVal()
}

func (o *ZObject) init(ctx phpv.Context) error {
	// Save the caller's location. During constant/property default resolution,
	// ctx.Loc() may change to the definition file. The [constant expression]
	// stack frame should show the call site (e.g., where "new Foo()" appears).
	callerLoc := ctx.Loc()

	// Resolve any pending CompileDelayed constants when the class is first used.
	// This ensures forward-referenced constants throw errors at instantiation time
	// if the referenced class/constant doesn't exist.
	if err := o.GetClass().(*ZClass).ResolveConstants(ctx); err != nil {
		return err
	}

	// Ensure static property defaults are resolved eagerly.
	// PHP resolves these when the class is first used (linked),
	// so errors like undefined constants in static defaults
	// should be thrown at instantiation time.
	if _, err := o.GetClass().(*ZClass).GetStaticProps(ctx); err != nil {
		// Add [constant expression] stack frame to match PHP behavior
		if ex, ok := err.(*phperr.PhpThrow); ok {
			AddConstantExpressionFrameAt(ex, callerLoc)
		}
		return err
	}

	// initialize object variables with default values

	class := o.GetClass().(*ZClass)
	lineage := []*ZClass{}
	for class != nil {
		lineage = append(lineage, class)
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}

	// PHP 8.4: Build a set of property names where a child class overrides
	// with a virtual hooked property (no default value). The parent's default
	// should NOT be inherited in this case.
	virtualOverrides := make(map[phpv.ZString]bool)
	// lineage[0] is the most-derived class (child), iterate from child to parent
	for i, cl := range lineage {
		if i == 0 {
			continue // skip the most-derived class itself, only check overrides
		}
		for _, p := range lineage[0].Props { // check child's props
			if p.Modifiers.IsStatic() || p.Modifiers.IsPrivate() {
				continue
			}
			if p.HasHooks && p.Default == nil {
				// Check if a parent has this property with a default
				for j := i; j < len(lineage); j++ {
					for _, pp := range lineage[j].Props {
						if pp.VarName == p.VarName && pp.Default != nil {
							virtualOverrides[p.VarName] = true
						}
					}
				}
			}
		}
		_ = cl
	}

	for _, class := range slices.Backward(lineage) {
		// Set compiling class for self::/parent:: resolution in property defaults
		ctx.Global().SetCompilingClass(class)
		for _, p := range class.Props {
			if p.Modifiers.IsStatic() {
				continue
			}
			// PHP 8.4: Virtual hooked properties have no backing store - skip initialization
			if p.IsVirtual() {
				if p.Modifiers.IsPrivate() {
					o.hasPrivate[p.VarName] = struct{}{}
				}
				continue
			}
			// PHP 8.4: If a child class overrides this property with a hooked property
			// that has no default value, skip the parent's default initialization.
			if virtualOverrides[p.VarName] && class != lineage[0] {
				continue
			}
			if p.Modifiers.IsPrivate() {
				// Private properties are stored ONLY under their mangled name
				// to avoid collisions with same-named properties in parent/child classes.
				k := getPrivatePropName(class, p.VarName)
				if p.Default != nil {
					// Resolve CompileDelayed defaults lazily
					if cd, ok := p.Default.(*phpv.CompileDelayed); ok {
						z, err := cd.Run(ctx)
						if err != nil {
							if ex, ok2 := err.(*phperr.PhpThrow); ok2 {
								AddConstantExpressionFrameAt(ex, callerLoc)
							}
							return err
						}
						p.Default = z.Value()
					}
					// Enforce typed property type checking on resolved defaults
				if p.TypeHint != nil {
					defVal := dupDefault(p.Default)
					if coerced, err := o.enforcePropertyType(ctx, p.VarName, p, defVal); err != nil {
						return err
					} else if coerced != nil {
						o.h.SetString(k, coerced)
					} else {
						o.h.SetString(k, defVal)
					}
				} else {
					o.h.SetString(k, dupDefault(p.Default))
				}
				} else if p.TypeHint == nil {
					// Untyped properties without default get null
					o.h.SetString(k, phpv.ZNULL.ZVal())
				}
				// Typed properties without default are "uninitialized" - don't set them
				o.hasPrivate[p.VarName] = struct{}{}
			} else {
				// Public/protected properties stored under bare name
				if p.Default != nil {
					// Resolve CompileDelayed defaults lazily
					if cd, ok := p.Default.(*phpv.CompileDelayed); ok {
						z, err := cd.Run(ctx)
						if err != nil {
							if ex, ok2 := err.(*phperr.PhpThrow); ok2 {
								AddConstantExpressionFrameAt(ex, callerLoc)
							}
							return err
						}
						p.Default = z.Value()
					}
					// Enforce typed property type checking on resolved defaults
				if p.TypeHint != nil {
					defVal := dupDefault(p.Default)
					if coerced, err := o.enforcePropertyType(ctx, p.VarName, p, defVal); err != nil {
						return err
					} else if coerced != nil {
						o.h.SetString(p.VarName, coerced)
					} else {
						o.h.SetString(p.VarName, defVal)
					}
				} else {
					o.h.SetString(p.VarName, dupDefault(p.Default))
				}
				} else if p.TypeHint == nil {
					// Untyped properties without default get null
					o.h.SetString(p.VarName, phpv.ZNULL.ZVal())
				}
				// Typed properties without default are "uninitialized" - don't set them
			}
		}
	}
	ctx.Global().SetCompilingClass(nil)

	return nil
}

func (o *ZObject) IterProps(ctx phpv.Context) iter.Seq[*phpv.ZClassProp] {
	return (&propIterator{ctx, o}).yield
}

type propIterator struct {
	ctx phpv.Context
	o   *ZObject
}

func (pi *propIterator) yield(yield func(*phpv.ZClassProp) bool) {
	o := pi.o
	ctx := pi.ctx
	shown := map[string]struct{}{}

	// Build lineage from current class to root
	var lineage []*ZClass
	class := o.GetClass().(*ZClass)
	for class != nil {
		lineage = append(lineage, class)
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}

	// Build a map of non-private property names to their most-derived version.
	// lineage[0] is the most-derived class, so iterating from child to parent
	// and keeping the first occurrence gives us the most-derived version.
	mostDerived := map[string]*phpv.ZClassProp{}
	for _, cl := range lineage {
		for _, p := range cl.Props {
			if p.Modifiers.IsStatic() {
				continue
			}
			if !p.Modifiers.IsPrivate() {
				if _, ok := mostDerived[p.VarName.String()]; !ok {
					mostDerived[p.VarName.String()] = p
				}
			}
		}
	}

	// Iterate from root to leaf (parent properties first) for correct ordering.
	// For non-private properties, yield the most-derived version at the position
	// where the property first appears in the hierarchy.
	for i := len(lineage) - 1; i >= 0; i-- {
		cl := lineage[i]
		for _, p := range cl.Props {
			if p.Modifiers.IsStatic() {
				continue
			}
			// PHP 8.4: Virtual hooked properties with no get hook (set-only) are
			// skipped in iteration. Virtual properties with a get hook are included
			// (the get hook is called to produce the value by the caller).
			if p.IsVirtual() && !o.h.HasString(p.VarName) {
				if p.GetHook == nil {
					// Set-only virtual property: skip
					if !p.Modifiers.IsPrivate() {
						shown[p.VarName.String()] = struct{}{}
					}
					continue
				}
				// Virtual property with get hook: include it in iteration
			}
			if !p.Modifiers.IsPrivate() {
				if _, ok := shown[p.VarName.String()]; ok {
					continue
				}
				// Skip non-typed properties that have been unset from the instance.
				// Typed properties are always shown (as "uninitialized(type)" in var_dump).
				if !o.h.HasString(p.VarName) {
					if p.TypeHint == nil {
						shown[p.VarName.String()] = struct{}{}
						continue
					}
				}
				shown[p.VarName.String()] = struct{}{}
				// Yield the most-derived version of this property
				if derived, ok := mostDerived[p.VarName.String()]; ok {
					p = derived
				}
			} else {
				// Skip private properties that have been unset (unless typed)
				propName := getPrivatePropName(cl, p.VarName)
				if !o.h.HasString(propName) {
					if p.TypeHint == nil {
						continue
					}
				}
			}
			if !yield(p) {
				return
			}
		}
	}
	for k := range o.h.NewIterator().Iterate(ctx) {
		key := k.AsString(ctx)
		// Skip mangled private property names (internal format: *ClassName:propName)
		if len(key) > 0 && key[0] == '*' {
			continue
		}
		if _, ok := shown[string(key)]; !ok {
			p := &phpv.ZClassProp{
				VarName: key,
			}
			if !yield(p) {
				break
			}
		}
	}
}

// HasPropValue returns true if the property has a value in the hash table.
// Returns false for typed properties that have not been initialized.
func (o *ZObject) HasPropValue(p *phpv.ZClassProp) bool {
	if p.Modifiers.IsPrivate() {
		class := o.Class.(*ZClass)
		for class != nil {
			for _, cp := range class.Props {
				if cp == p {
					k := getPrivatePropName(class, p.VarName)
					return o.h.HasString(k)
				}
			}
			parent := class.GetParent()
			if parent == nil {
				break
			}
			class = parent.(*ZClass)
		}
	}
	return o.h.HasString(p.VarName)
}

// GetPropValue returns the value for a class property, handling the mangled
// name lookup for private properties.
func (o *ZObject) GetPropValue(p *phpv.ZClassProp) *phpv.ZVal {
	if p.Modifiers.IsPrivate() {
		// Find the declaring class by matching the exact prop pointer
		class := o.Class.(*ZClass)
		for class != nil {
			for _, cp := range class.Props {
				if cp == p {
					k := getPrivatePropName(class, p.VarName)
					return o.h.GetString(k)
				}
			}
			parent := class.GetParent()
			if parent == nil {
				break
			}
			class = parent.(*ZClass)
		}
	}
	return o.h.GetString(p.VarName)
}

// SetPropValueDirect sets a property value directly, bypassing visibility checks.
// For private properties, it finds the declaring class and uses the mangled name.
// This is used by Reflection to set properties regardless of visibility.
func (o *ZObject) SetPropValueDirect(prop *phpv.ZClassProp, value *phpv.ZVal) error {
	if prop.Modifiers.IsPrivate() {
		class := o.Class.(*ZClass)
		for class != nil {
			for _, cp := range class.Props {
				if cp == prop {
					k := getPrivatePropName(class, prop.VarName)
					return o.h.SetString(k, value)
				}
			}
			parent := class.GetParent()
			if parent == nil {
				break
			}
			class = parent.(*ZClass)
		}
	}
	return o.h.SetString(prop.VarName, value)
}

// GetPropValueOrHook returns the value for a class property. For virtual
// hooked properties with a get hook, it calls the get hook. For backed
// properties or properties without hooks, it returns the hash table value.
// Returns (value, hasValue, error).
func (o *ZObject) GetPropValueOrHook(ctx phpv.Context, p *phpv.ZClassProp) (*phpv.ZVal, bool, error) {
	// If this is a virtual hooked property with a get hook, call the hook
	if p.HasHooks && p.GetHook != nil && p.IsVirtual() && !o.HasPropValue(p) {
		val, err := o.runGetHook(ctx, p.VarName, p)
		if err != nil {
			return nil, false, err
		}
		return val, true, nil
	}
	// For backed/normal properties, read from hash table
	if o.HasPropValue(p) {
		return o.GetPropValue(p), true, nil
	}
	return nil, false, nil
}

// GetDeclClassName returns the declaring class name for a private property.
func (o *ZObject) GetDeclClassName(p *phpv.ZClassProp) phpv.ZString {
	class := o.Class.(*ZClass)
	for class != nil {
		for _, cp := range class.Props {
			if cp == p {
				return class.GetName()
			}
		}
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}
	return o.Class.GetName()
}

func (o *ZObject) implementsArrayAccess() bool {
	return o.Class.Implements(ArrayAccess)
}

func (o *ZObject) CallMethod(ctx phpv.Context, methodName string, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	m, err := o.GetMethod(phpv.ZString(methodName), ctx)
	if err != nil {
		return nil, err
	}
	return ctx.CallZVal(ctx, m, args, o)
}

func (o *ZObject) OffsetGet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, error) {
	if !o.implementsArrayAccess() {
		return nil, ThrowError(ctx, Error, fmt.Sprintf("Cannot use object of type %s as array", o.Class.GetName()))
	}
	return o.CallMethod(ctx, "offsetGet", key.ZVal())
}

func (o *ZObject) OffsetSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error {
	if !o.implementsArrayAccess() {
		return ThrowError(ctx, Error, fmt.Sprintf("Cannot use object of type %s as array", o.Class.GetName()))
	}
	var keyZVal *phpv.ZVal
	if key == nil {
		keyZVal = phpv.ZNULL.ZVal()
	} else {
		keyZVal = key.ZVal()
	}
	_, err := o.CallMethod(ctx, "offsetSet", keyZVal, value)
	return err
}

func (o *ZObject) OffsetExists(ctx phpv.Context, key phpv.Val) (bool, error) {
	if !o.implementsArrayAccess() {
		return false, ThrowError(ctx, Error, fmt.Sprintf("Cannot use object of type %s as array", o.Class.GetName()))
	}
	result, err := o.CallMethod(ctx, "offsetExists", key.ZVal())
	if err != nil {
		return false, err
	}
	return bool(result.AsBool(ctx)), nil
}

func (o *ZObject) OffsetUnset(ctx phpv.Context, key phpv.Val) error {
	if !o.implementsArrayAccess() {
		return ThrowError(ctx, Error, fmt.Sprintf("Cannot use object of type %s as array", o.Class.GetName()))
	}
	_, err := o.CallMethod(ctx, "offsetUnset", key.ZVal())
	return err
}

func (o *ZObject) OffsetCheck(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, bool, error) {
	exists, err := o.OffsetExists(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	val, err := o.OffsetGet(ctx, key)
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// OffsetGetReturnsByRef checks whether the ArrayAccess offsetGet method
// on this object is declared with a return-by-reference signature (&offsetGet).
// When true, indirect modifications (++, +=, etc.) go through the reference
// and actually work, so the "Indirect modification has no effect" notice
// should be suppressed.
func (o *ZObject) OffsetGetReturnsByRef() bool {
	class := o.GetClass().(*ZClass)
	m, ok := class.Methods["offsetget"]
	if !ok {
		return false
	}
	if rr, ok2 := m.Method.(interface{ ReturnsByRef() bool }); ok2 {
		return rr.ReturnsByRef()
	}
	return false
}

func (o *ZObject) GetMethod(method phpv.ZString, ctx phpv.Context) (phpv.Callable, error) {
	class := o.GetClass().(*ZClass)
	m, ok := class.Methods[method.ToLower()]
	if !ok {
		m, ok = class.Methods["__call"]
		if ok {
			return &callCatcher{phpv.CallableVal{}, method, m.Method}, nil
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", o.Class.GetName(), method)
	}
	// Note: #[\Deprecated] check for user methods is handled by ZClosure.Call()
	// which fires when the method body is actually invoked.
	// TODO check method access
	return m.Method, nil
}

// checkMethodDeprecated emits a deprecation warning if a method has #[\Deprecated]
func checkMethodDeprecated(ctx phpv.Context, class *ZClass, m *phpv.ZClassMethod) {
	for _, attr := range m.Attributes {
		if attr.ClassName == "Deprecated" {
			msg := formatDeprecatedMsg("Method", string(class.GetName())+"::"+string(m.Name)+"()", attr)
			ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true))
			return
		}
	}
}

// formatDeprecatedMsg formats a deprecation message from a #[\Deprecated] attribute.
func formatDeprecatedMsg(label, name string, attr *phpv.ZAttribute) string {
	msg := fmt.Sprintf("%s %s is deprecated", label, name)

	// Coerce scalar types to string, matching Deprecated constructor behavior
	var message, since string
	if len(attr.Args) > 0 && !attr.Args[0].IsNull() {
		message = attr.Args[0].String()
	}
	if len(attr.Args) > 1 && !attr.Args[1].IsNull() {
		since = attr.Args[1].String()
	}

	if since != "" {
		msg += " since " + since
	}
	if message != "" {
		msg += ", " + message
	}

	return msg
}

func (o *ZObject) HasProp(ctx phpv.Context, key phpv.Val) (bool, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return false, err
	}

	keyStr := key.(phpv.ZString)

	// Lazy object: trigger initialization on isset (unless skipped)
	if o.IsLazy() && !o.IsPropertySkippedForLazy(keyStr) {
		if err := o.TriggerLazyInitForProp(ctx, keyStr); err != nil {
			return false, err
		}
		if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil {
			target := o.ResolveProxy()
			if target.IsLazy() {
				if err := target.TriggerLazyInit(ctx); err != nil {
					return false, err
				}
				target = target.ResolveProxy()
			}
			return target.HasProp(ctx, keyStr)
		}
	} else if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil && !o.IsPropertySkippedForLazy(keyStr) {
		target := o.ResolveProxy()
		if target.IsLazy() {
			if err := target.TriggerLazyInit(ctx); err != nil {
				return false, err
			}
			target = target.ResolveProxy()
		}
		return target.HasProp(ctx, keyStr)
	}

	// Note: isset() does NOT emit "Accessing static property as non-static" notice
	// (unlike ObjectGet/ObjectSet which do). This is PHP behavior.

	// PHP 8.4: Property hooks - isset() on a hooked property calls the get hook
	// and checks if the result is non-null. Write-only properties throw Error.
	// When inside ANY hook for this property, bypass hook dispatch.
	issetInsideHook := (o.getHookGuard != nil && o.getHookGuard[keyStr]) ||
		(o.setHookGuard != nil && o.setHookGuard[keyStr])
	if !issetInsideHook {
		if prop := o.findPropWithHook(keyStr); prop != nil {
			if prop.GetHook != nil {
				// Call the get hook and check if result is non-null
				result, err := o.runGetHook(ctx, keyStr, prop)
				if err != nil {
					return false, err
				}
				if result == nil || result.IsNull() {
					return false, nil
				}
				return true, nil
			}
			// Write-only virtual: set hook but no get hook, not backed, and no backing value
			if prop.SetHook != nil && prop.GetHook == nil && !prop.IsBacked && !o.h.HasString(keyStr) {
				return false, ThrowError(ctx, Error,
					fmt.Sprintf("Property %s::$%s is write-only", o.Class.GetName(), keyStr))
			}
		}
	}

	// Check if property is visible from the calling context.
	// If the property is declared private/protected and the caller doesn't have
	// access, we should fall through to __isset rather than returning true.
	propVisible := true
	if o.isPropertyHidden(ctx, keyStr) {
		propVisible = false
	}

	if propVisible {
		if _, ok := o.hasPrivate[keyStr]; ok {
			resolveClass := o.resolvePrivateClass(ctx, keyStr)
			propName := getPrivatePropName(resolveClass, keyStr)
			if o.h.HasString(propName) {
				return true, nil
			}
		}

		if o.h.HasString(keyStr) {
			return true, nil
		}
	}

	// Property not found; check for class-level property isset handler before __isset
	if h := FindPropHandlers(o.Class); h != nil && h.HandlePropIsset != nil {
		if result, handled, err := h.HandlePropIsset(ctx, o, keyStr); handled || err != nil {
			return result, err
		}
	}

	// Property not found or not visible, try __isset magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__isset"]; ok {
		// Guard against infinite recursion
		if o.issetGuard == nil {
			o.issetGuard = make(map[phpv.ZString]bool)
		}
		if o.issetGuard[keyStr] {
			return false, nil
		}
		o.issetGuard[keyStr] = true
		result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
		delete(o.issetGuard, keyStr)
		if err != nil {
			return false, err
		}
		return bool(result.AsBool(ctx)), nil
	}

	return false, nil
}

// FindIssetDimHandler walks up the class hierarchy looking for a HandleIssetDim handler.
// Returns the handler function or nil if none found.
func FindIssetDimHandler(cls phpv.ZClass) func(phpv.Context, phpv.ZObject, *phpv.ZVal) (bool, error) {
	for cls != nil {
		if h := cls.Handlers(); h != nil && h.HandleIssetDim != nil {
			return h.HandleIssetDim
		}
		cls = cls.GetParent()
	}
	return nil
}

// FindReadDimHandler walks up the class hierarchy looking for a HandleReadDim handler.
// Returns the handler function or nil if none found.
func FindReadDimHandler(cls phpv.ZClass) func(phpv.Context, phpv.ZObject, *phpv.ZVal) (*phpv.ZVal, error) {
	for cls != nil {
		if h := cls.Handlers(); h != nil && h.HandleReadDim != nil {
			return h.HandleReadDim
		}
		cls = cls.GetParent()
	}
	return nil
}

// FindPropHandlers walks up the class hierarchy looking for a class that has
// property access handlers defined. Returns the first non-nil ZClassHandlers
// that has at least one property handler, or nil if none found.
func FindPropHandlers(cls phpv.ZClass) *phpv.ZClassHandlers {
	for cls != nil {
		if h := cls.Handlers(); h != nil {
			if h.HandlePropGet != nil || h.HandlePropSet != nil || h.HandlePropIsset != nil || h.HandlePropUnset != nil {
				return h
			}
		}
		cls = cls.GetParent()
	}
	return nil
}

// isPropertyHidden returns true if the property is declared with restricted visibility
// (private/protected) and the current calling context doesn't have access.
// Used by HasProp to decide whether to fall through to __isset.
func (o *ZObject) isPropertyHidden(ctx phpv.Context, keyStr phpv.ZString) bool {
	callerClass := ctx.Class()

	// Check caller's own class first
	if callerClass != nil {
		if callerZClass, ok := callerClass.(*ZClass); ok {
			if prop, ok := callerZClass.GetProp(keyStr); ok && prop.Modifiers.IsPrivate() && !prop.Modifiers.IsStatic() {
				return false // caller's own private property - visible
			}
		}
	}

	// Walk the class hierarchy looking for the property declaration
	class := o.Class.(*ZClass)
	for class != nil {
		if prop, ok := class.GetProp(keyStr); ok {
			if prop.Modifiers.IsPrivate() {
				return callerClass == nil || callerClass.GetName() != class.GetName()
			}
			if prop.Modifiers.IsProtected() {
				if callerClass == nil {
					return true
				}
				return !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass)
			}
			return false // public
		}
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}
	return false // no declaration found, not hidden
}

// checkPropertyVisibility checks if the caller context has access to a property.
// Returns nil if access is allowed, or an error to throw.
func (o *ZObject) checkPropertyVisibility(ctx phpv.Context, keyStr phpv.ZString, action string) error {
	callerClass := ctx.Class()

	// First, check if the caller's own class declares a private property with this name.
	// Private properties are not virtual: if class A has private $p and class B (extends A)
	// also has private $p, A's methods should always access A's $p. So if the caller's class
	// declares this property as private, access is always allowed (it's their own property).
	if callerClass != nil {
		if callerZClass, ok := callerClass.(*ZClass); ok {
			// Only check the caller's OWN declared props (not walking hierarchy).
			// D extending C with private $p: D should NOT get blanket access.
			if prop, ok := getOwnProp(callerZClass, keyStr); ok && prop.Modifiers.IsPrivate() && !prop.Modifiers.IsStatic() {
				return nil
			}
		}
	}

	// Walk the class hierarchy, checking each class's OWN declared props
	// (not using GetProp which walks the hierarchy internally).
	class := o.Class.(*ZClass)
	concreteClass := class
	for class != nil {
		for _, prop := range class.Props {
			if prop.VarName != keyStr {
				continue
			}
			if prop.Modifiers.IsPrivate() {
				if callerClass != nil && callerClass.GetName() == class.GetName() {
					return nil // caller is the declaring class, allowed
				}
				// Private property from a parent class is invisible to outsiders.
				// Skip it and continue looking in parent classes.
				if class != concreteClass {
					goto nextClass
				}
				return ThrowError(ctx, Error, fmt.Sprintf("Cannot access private property %s::$%s", class.GetName(), keyStr))
			} else if prop.Modifiers.IsProtected() {
				if callerClass == nil {
					return ThrowError(ctx, Error, fmt.Sprintf("Cannot access protected property %s::$%s", o.Class.GetName(), keyStr))
				}
				if !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) &&
					!sharesProtectedPropertyPrototype(callerClass, concreteClass, keyStr) {
					return ThrowError(ctx, Error, fmt.Sprintf("Cannot access protected property %s::$%s", o.Class.GetName(), keyStr))
				}
			}
			return nil
		}
	nextClass:
		if class.Extends == nil {
			break
		}
		class = class.Extends
	}
	return nil
}

// IsReadonlyProperty checks if a property is declared as readonly in the class hierarchy.
// Used for blocking indirect modifications (e.g. $obj->readonlyProp[] = val).
// Enum properties (name, value) are always treated as readonly.
func (o *ZObject) IsReadonlyProperty(keyStr phpv.ZString) bool {
	// Enum properties are implicitly readonly
	if zc, ok := o.GetClass().(*ZClass); ok && zc.Type.Has(phpv.ZClassTypeEnum) {
		if keyStr == "name" || (keyStr == "value" && zc.EnumBackingType != 0) {
			return true
		}
	}
	class := o.GetClass().(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == keyStr && prop.Modifiers.IsReadonly() {
				return true
			}
		}
	}
	return false
}

// IsReadonlyPropertyInitialized checks if a readonly property has been initialized.
// Enum properties (name, value) are always considered initialized.
func (o *ZObject) IsReadonlyPropertyInitialized(keyStr phpv.ZString) bool {
	// Enum properties are always initialized
	if zc, ok := o.GetClass().(*ZClass); ok && zc.Type.Has(phpv.ZClassTypeEnum) {
		if keyStr == "name" || (keyStr == "value" && zc.EnumBackingType != 0) {
			return true
		}
	}
	return o.readonlyInit != nil && o.readonlyInit[keyStr]
}

// MarkReadonlyInitialized marks a readonly property as initialized.
// Used by native constructors that set properties via HashTable directly.
func (o *ZObject) MarkReadonlyInitialized(keyStr phpv.ZString) {
	if o.readonlyInit == nil {
		o.readonlyInit = make(map[phpv.ZString]bool)
	}
	o.readonlyInit[keyStr] = true
}

// checkReadonlyWrite checks if a property is readonly and already initialized.
// Returns an error if the property cannot be written to.
func (o *ZObject) checkReadonlyWrite(ctx phpv.Context, keyStr phpv.ZString) error {
	class := o.GetClass().(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == keyStr && prop.Modifiers.IsReadonly() {
				// Readonly property found. Check if it's already initialized.
				if o.readonlyInit != nil && o.readonlyInit[keyStr] {
					return ThrowError(ctx, Error,
						fmt.Sprintf("Cannot modify readonly property %s::$%s", class.GetName(), keyStr))
				}

				// First write — mark as initialized
				if o.readonlyInit == nil {
					o.readonlyInit = make(map[phpv.ZString]bool)
				}
				o.readonlyInit[keyStr] = true
				return nil
			}
		}
	}
	return nil
}

// MarkReadonlyInit marks a readonly property as initialized (for use by native constructors
// that set properties directly on the hash table without going through ObjectSet).
func (o *ZObject) MarkReadonlyInit(key phpv.ZString) {
	if o.readonlyInit == nil {
		o.readonlyInit = make(map[phpv.ZString]bool)
	}
	o.readonlyInit[key] = true
}

// checkSetVisibility checks PHP 8.4 asymmetric visibility for property writes.
// If a property has SetModifiers != 0, the write visibility may be more restrictive
// than the read visibility. For example, "public private(set)" allows anyone to read
// but only the declaring class to write.
// The isUnset parameter controls whether error messages say "Cannot unset" vs "Cannot modify".
func (o *ZObject) checkSetVisibility(ctx phpv.Context, keyStr phpv.ZString, isUnset ...bool) error {
	verb := "modify"
	if len(isUnset) > 0 && isUnset[0] {
		verb = "unset"
	}
	class := o.Class.(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName != keyStr || prop.Modifiers.IsStatic() {
				continue
			}
			if prop.SetModifiers == 0 {
				// PHP 8.4: public readonly without explicit set visibility has
				// implicit protected(set) scope for the initial write.
				// Only enforce this for properties that haven't been initialized yet.
				// Already-initialized readonly properties will get the standard
				// "Cannot modify readonly property" error from checkReadonlyWrite.
				if prop.Modifiers.IsReadonly() && prop.Modifiers.IsPublic() {
					alreadyInit := o.readonlyInit != nil && o.readonlyInit[keyStr]
					if !alreadyInit {
						callerClass := ctx.Class()
						if callerClass == nil || (!callerClass.InstanceOf(cur) && !cur.InstanceOf(callerClass) &&
						!sharesProtectedPropertyPrototype(callerClass, class, keyStr)) {
							return ThrowError(ctx, Error,
								fmt.Sprintf("Cannot %s protected(set) readonly property %s::$%s from %s",
									verb, cur.GetName(), keyStr, scopeName(callerClass)))
						}
					}
				}
				return nil
			}
			callerClass := ctx.Class()
			if prop.SetModifiers.IsPrivate() {
				// Only the declaring class can write
				if callerClass != nil && callerClass.GetName() == cur.GetName() {
					return nil
				}
				return ThrowError(ctx, Error,
					fmt.Sprintf("Cannot %s private(set) property %s::$%s from %s",
						verb, cur.GetName(), keyStr, scopeName(callerClass)))
			}
			if prop.SetModifiers.IsProtected() {
				// The declaring class and subclasses can write.
				// Also allow access if the caller and object share a common ancestor
				// that declares the property (prototype-based scoping, GH-19044).
				if callerClass != nil && (callerClass.InstanceOf(cur) || cur.InstanceOf(callerClass) ||
					sharesProtectedPropertyPrototype(callerClass, class, keyStr)) {
					return nil
				}
				readonlyStr := ""
				if prop.Modifiers.IsReadonly() {
					readonlyStr = " readonly"
				}
				return ThrowError(ctx, Error,
					fmt.Sprintf("Cannot %s protected(set)%s property %s::$%s from %s",
						verb, readonlyStr, cur.GetName(), keyStr, scopeName(callerClass)))
			}
			return nil
		}
	}
	return nil
}

// sharesProtectedPropertyPrototype checks if two classes share a common ancestor
// that declares a property. In PHP 8.4+, protected property scoping is based on
// the property's prototype - the most ancestral class that declares the property.
// If both the caller and the object's class descend from a class that declares
// the property, access is allowed even for sibling classes.
func sharesProtectedPropertyPrototype(callerClass phpv.ZClass, objectClass *ZClass, keyStr phpv.ZString) bool {
	// Find the most ancestral class that declares this property
	var findPrototype func(c *ZClass) *ZClass
	findPrototype = func(c *ZClass) *ZClass {
		var deepest *ZClass
		for cur := c; cur != nil; cur = cur.Extends {
			for _, prop := range cur.Props {
				if prop.VarName == keyStr && !prop.Modifiers.IsPrivate() {
					deepest = cur
				}
			}
		}
		return deepest
	}

	proto := findPrototype(objectClass)
	if proto == nil {
		return false
	}

	// Check if the caller class is an instance of the prototype
	return callerClass.InstanceOf(proto)
}

// findPropWithHook looks up a class property by name, walking the class hierarchy.
// Returns the ZClassProp if found and it has hooks, nil otherwise.
func (o *ZObject) findPropWithHook(keyStr phpv.ZString) *phpv.ZClassProp {
	class := o.Class.(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == keyStr && prop.HasHooks {
				return prop
			}
		}
	}
	return nil
}

// runGetHook executes a property get hook in the context of this object.
// It uses CallZVal with a HookCallable to create a proper FuncContext with $this bound.
func (o *ZObject) runGetHook(ctx phpv.Context, keyStr phpv.ZString, prop *phpv.ZClassProp) (*phpv.ZVal, error) {
	hook := prop.GetHook
	// Set recursion guard so $this->propName inside the hook accesses the backing value
	if o.getHookGuard == nil {
		o.getHookGuard = make(map[phpv.ZString]bool)
	}
	o.getHookGuard[keyStr] = true
	defer delete(o.getHookGuard, keyStr)

	// Create a callable wrapper for the hook body so CallZVal creates a
	// proper FuncContext with $this bound to the object. Wrap in MethodCallable
	// so the class context is set, allowing access to private/protected members.
	hookCallable := &phpv.MethodCallable{
		Callable: &phpv.HookCallable{
			Hook:     hook,
			HookName: fmt.Sprintf("%s::$%s::get", o.Class.GetName(), keyStr),
		},
		Class: o.Class,
	}

	result, err := ctx.CallZVal(ctx, hookCallable, nil, o)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = phpv.ZNULL.ZVal()
	}

	// Type-check the return value against the property type
	if prop.TypeHint != nil && result != nil && !result.IsNull() {
		hint := prop.TypeHint
		if hint.Check(ctx, result) {
			// For scalar types, coerce the value to the exact type
			hintType := hint.Type()
			valType := result.GetType()
			if hintType != phpv.ZtMixed && hintType != phpv.ZtObject && valType != hintType {
				if hintType == phpv.ZtInt && valType == phpv.ZtFloat {
					v, err := phpv.FloatToIntImplicit(ctx, result.Value().(phpv.ZFloat))
					if err != nil {
						return nil, err
					}
					result = v.ZVal()
				} else if coerced, err := result.Value().AsVal(ctx, hintType); err == nil && coerced != nil {
					result = coerced.ZVal()
				}
			}
		} else {
			returnTypeName := phpv.ZValTypeName(result)
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("%s::$%s::get(): Return value must be of type %s, %s returned",
					o.Class.GetName(), keyStr, hint.String(), returnTypeName))
		}
	} else if prop.TypeHint != nil && (result == nil || result.IsNull()) && !prop.TypeHint.IsNullable() {
		// Null returned for non-nullable type
		return nil, ThrowError(ctx, TypeError,
			fmt.Sprintf("%s::$%s::get(): Return value must be of type %s, null returned",
				o.Class.GetName(), keyStr, prop.TypeHint.String()))
	}

	return result, nil
}

// runSetHook executes a property set hook in the context of this object.
// It uses CallZVal with a HookCallable to create a proper FuncContext with $this
// and the $value parameter available.
// For short arrow set hooks (set => expr), the result is assigned to the backing value.
func (o *ZObject) runSetHook(ctx phpv.Context, keyStr phpv.ZString, prop *phpv.ZClassProp, value *phpv.ZVal) error {
	// Set recursion guard so $this->propName inside the hook accesses the backing value
	if o.setHookGuard == nil {
		o.setHookGuard = make(map[phpv.ZString]bool)
	}
	o.setHookGuard[keyStr] = true
	defer delete(o.setHookGuard, keyStr)

	paramName := prop.SetParam
	if paramName == "" {
		paramName = "value"
	}

	// Create a callable wrapper that declares the set parameter.
	// The hook body references $value (or custom param name) as a local variable.
	// Wrap in MethodCallable so CallZVal sets the class context, allowing
	// the hook to access private/protected properties of the declaring class.
	hookCallable := &phpv.MethodCallable{
		Callable: &phpv.HookCallable{
			Hook:     prop.SetHook,
			HookName: fmt.Sprintf("%s::$%s::set", o.Class.GetName(), keyStr),
			Params: []*phpv.FuncArg{
				{VarName: paramName},
			},
		},
		Class: o.Class,
	}

	result, err := ctx.CallZVal(ctx, hookCallable, []*phpv.ZVal{value}, o)
	if err != nil {
		return err
	}

	// For short arrow set hooks (set => expr), the expression result is assigned
	// to the backing property. Block set hooks (set { ... }) assign to
	// $this->prop directly inside the body; their return value is ignored.
	// Block hooks compile to phpv.Runnables (a []Runnable); arrow hooks compile
	// to a single expression Runnable.
	_, isBlock := prop.SetHook.(phpv.Runnables)
	if !isBlock && result != nil && !result.IsNull() {
		o.objectSetBacking(keyStr, result)
	}

	return nil
}

// objectSetBacking directly sets the backing value of a property in the hash table,
// bypassing hooks and visibility checks. Used by set hooks to store the final value.
func (o *ZObject) objectSetBacking(keyStr phpv.ZString, value *phpv.ZVal) {
	if _, ok := o.hasPrivate[keyStr]; ok {
		// For private properties, we need to find the mangled name
		class := o.Class.(*ZClass)
		for cur := class; cur != nil; cur = cur.Extends {
			for _, prop := range cur.Props {
				if prop.VarName == keyStr && prop.Modifiers.IsPrivate() {
					propName := getPrivatePropName(cur, keyStr)
					o.h.SetString(propName, value)
					return
				}
			}
		}
	}
	o.h.SetString(keyStr, value)
}

// RunParentGetHook executes a parent class's get hook on this object.
// Used by parent::$prop::get() in property hooks.
func (o *ZObject) RunParentGetHook(ctx phpv.Context, keyStr phpv.ZString, hook phpv.Runnable, declClass *ZClass) (*phpv.ZVal, error) {
	// Set recursion guard
	if o.getHookGuard == nil {
		o.getHookGuard = make(map[phpv.ZString]bool)
	}
	o.getHookGuard[keyStr] = true
	defer delete(o.getHookGuard, keyStr)

	hookCallable := &phpv.MethodCallable{
		Callable: &phpv.HookCallable{
			Hook:     hook,
			HookName: fmt.Sprintf("%s::$%s::get", declClass.GetName(), keyStr),
		},
		Class: declClass,
	}

	result, err := ctx.CallZVal(ctx, hookCallable, nil, o)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = phpv.ZNULL.ZVal()
	}
	return result, nil
}

// RunParentSetHook executes a parent class's set hook on this object.
// Used by parent::$prop::set() in property hooks.
func (o *ZObject) RunParentSetHook(ctx phpv.Context, keyStr phpv.ZString, prop *phpv.ZClassProp, value *phpv.ZVal, declClass *ZClass) error {
	// Set recursion guard
	if o.setHookGuard == nil {
		o.setHookGuard = make(map[phpv.ZString]bool)
	}
	o.setHookGuard[keyStr] = true
	defer delete(o.setHookGuard, keyStr)

	paramName := prop.SetParam
	if paramName == "" {
		paramName = "value"
	}

	hookCallable := &phpv.MethodCallable{
		Callable: &phpv.HookCallable{
			Hook:     prop.SetHook,
			HookName: fmt.Sprintf("%s::$%s::set", declClass.GetName(), keyStr),
			Params: []*phpv.FuncArg{
				{VarName: paramName},
			},
		},
		Class: declClass,
	}

	result, err := ctx.CallZVal(ctx, hookCallable, []*phpv.ZVal{value}, o)
	if err != nil {
		return err
	}

	// For short arrow set hooks, the result is assigned to the backing property
	_, isBlock := prop.SetHook.(phpv.Runnables)
	if !isBlock && result != nil && !result.IsNull() {
		o.objectSetBacking(keyStr, result)
	}

	return nil
}

// ReadParentBacking reads the backing value of a parent property directly.
// Used by parent::$prop::get() when the parent has no get hook.
func (o *ZObject) ReadParentBacking(ctx phpv.Context, keyStr phpv.ZString, declClass *ZClass) (*phpv.ZVal, error) {
	// Check private properties with name mangling
	if _, ok := o.hasPrivate[keyStr]; ok {
		propName := getPrivatePropName(declClass, keyStr)
		if o.h.HasString(propName) {
			v := o.h.GetString(propName)
			return phpv.NewZVal(v.Value()), nil
		}
	}

	if o.h.HasString(keyStr) {
		v := o.h.GetString(keyStr)
		return phpv.NewZVal(v.Value()), nil
	}

	// Check for uninitialized typed property - use the child class name (object's actual class)
	for _, p := range declClass.Props {
		if p.VarName == keyStr && p.TypeHint != nil {
			return nil, ThrowError(ctx, Error,
				fmt.Sprintf("Typed property %s::$%s must not be accessed before initialization",
					o.Class.GetName(), keyStr))
		}
	}

	return phpv.ZNULL.ZVal(), nil
}

// WriteParentBacking writes directly to the backing value of a parent property.
// Used by parent::$prop::set() when the parent has no set hook.
func (o *ZObject) WriteParentBacking(ctx phpv.Context, keyStr phpv.ZString, value *phpv.ZVal, declClass *ZClass) {
	o.objectSetBacking(keyStr, value)
}

// ScopeName returns a human-readable scope name for error messages.
func ScopeName(class phpv.ZClass) string {
	if class == nil {
		return "global scope"
	}
	return fmt.Sprintf("scope %s", class.GetName())
}

// scopeName is the package-internal alias for ScopeName.
func scopeName(class phpv.ZClass) string {
	return ScopeName(class)
}

func (o *ZObject) ObjectGet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	keyStr := key.(phpv.ZString)

	// Lazy object: trigger initialization on property access (unless skipped)
	if o.IsLazy() && !o.IsPropertySkippedForLazy(keyStr) {
		if err := o.TriggerLazyInitForProp(ctx, keyStr); err != nil {
			return nil, err
		}
		// After proxy init, delegate to the real instance
		if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil {
			target := o.ResolveProxy()
			if target.IsLazy() {
				if err := target.TriggerLazyInit(ctx); err != nil {
					return nil, err
				}
				target = target.ResolveProxy()
			}
			return target.ObjectGet(ctx, keyStr)
		}
	} else if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil && !o.IsPropertySkippedForLazy(keyStr) {
		target := o.ResolveProxy()
		if target.IsLazy() {
			if err := target.TriggerLazyInit(ctx); err != nil {
				return nil, err
			}
			target = target.ResolveProxy()
		}
		return target.ObjectGet(ctx, keyStr)
	}

	// Check if accessing a static property as non-static
	o.checkStaticPropertyAccess(ctx, keyStr)

	// Check property visibility. If the property is not visible but __get exists,
	// PHP calls __get instead of throwing the visibility error.
	visErr := o.checkPropertyVisibility(ctx, keyStr, "access")
	if visErr != nil {
		// Before returning the visibility error, check if __get is available
		class := o.GetClass().(*ZClass)
		if m, ok := class.Methods["__get"]; ok {
			if o.getGuard == nil {
				o.getGuard = make(map[phpv.ZString]bool)
			}
			if !o.getGuard[keyStr] {
				o.getGuard[keyStr] = true
				result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
				delete(o.getGuard, keyStr)
				if result == nil && err == nil {
					result = phpv.ZNULL.ZVal()
				}
				return result, err
			}
		}
		return nil, visErr
	}

	// Check for property get hook (PHP 8.4) - only if not inside ANY hook for this property.
	// When inside either get or set hook, $this->prop accesses the backing store directly.
	getInsideHook := (o.getHookGuard != nil && o.getHookGuard[keyStr]) ||
		(o.setHookGuard != nil && o.setHookGuard[keyStr])
	if !getInsideHook {
		if prop := o.findPropWithHook(keyStr); prop != nil {
			if prop.GetHook != nil {
				return o.runGetHook(ctx, keyStr, prop)
			}
			// Set-only virtual property without a backing value: reading throws Error.
			// Backed properties (IsBacked=true) fall through to normal property lookup
			// (which may throw "uninitialized typed property" if no value has been set yet).
			if prop.SetHook != nil && prop.GetHook == nil && !prop.IsBacked && !o.h.HasString(keyStr) {
				return nil, ThrowError(ctx, Error,
					fmt.Sprintf("Property %s::$%s is write-only", o.Class.GetName(), keyStr))
			}
		}
	}

	if _, ok := o.hasPrivate[keyStr]; ok {
		// Private properties are not virtual. If the caller's class declares a private
		// property with this name, resolve to the caller's copy, not the object's class copy.
		resolveClass := o.resolvePrivateClass(ctx, keyStr)
		propName := getPrivatePropName(resolveClass, keyStr)
		if o.h.HasString(propName) {
			v := o.h.GetString(propName)
			// Return a detached snapshot so in-place mutations to the hash
			// entry don't retroactively change already-read values (PHP semantics).
			return phpv.NewZVal(v.Value()), nil
		}
	}

	if o.h.HasString(keyStr) {
		v := o.h.GetString(keyStr)
		// Return the actual ZVal pointer from the hash table so that
		// by-reference access (e.g., func(&$obj->prop)) can modify
		// the property in-place. PHP's COW semantics handle the case
		// where the value is assigned to a separate variable.
		return v, nil
	}

	// Check for uninitialized typed property - throws Error instead of calling __get,
	// UNLESS the property was explicitly unset AND the class has __get (fallback allowed).
	if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil {
		// If the property was explicitly unset and __get exists, allow __get fallback
		explicitlyUnset := o.typedPropUnset != nil && (*o.typedPropUnset)[keyStr]
		hasGetMethod := false
		if zc, ok2 := o.Class.(*ZClass); ok2 {
			_, hasGetMethod = zc.Methods["__get"]
		}
		if !(explicitlyUnset && hasGetMethod) {
			// Find the declaring class for the error message
			declClass := o.Class.GetName()
			if zc, ok := o.Class.(*ZClass); ok {
				for cur := zc; cur != nil; cur = cur.Extends {
					for _, cp := range cur.Props {
						if cp.VarName == keyStr {
							declClass = cur.GetName()
							goto foundDecl
						}
					}
				}
			}
		foundDecl:
			return nil, ThrowError(ctx, Error,
				fmt.Sprintf("Typed property %s::$%s must not be accessed before initialization", declClass, keyStr))
		}
	}

	// Property not found; check for class-level property handler before __get
	// Walk up the class hierarchy to find handlers (subclasses inherit parent handlers)
	if h := FindPropHandlers(o.Class); h != nil && h.HandlePropGet != nil {
		if result, err := h.HandlePropGet(ctx, o, keyStr); result != nil || err != nil {
			return result, err
		}
		// (nil, nil) means fall through to normal handling
	}

	// Property not found, try __get magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__get"]; ok {
		if o.getGuard == nil {
			o.getGuard = make(map[phpv.ZString]bool)
		}
		if !o.getGuard[keyStr] {
			o.getGuard[keyStr] = true
			result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
			delete(o.getGuard, keyStr)
			if result == nil && err == nil {
				result = phpv.ZNULL.ZVal()
			}
			if err != nil {
				return result, err
			}
			// Check type compatibility of __get() return value for unset typed properties
			if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil {
				if o.typedPropUnset != nil && (*o.typedPropUnset)[keyStr] {
					hint := prop.TypeHint
					if result.IsNull() && !hint.IsNullable() {
						return nil, ThrowError(ctx, TypeError,
							fmt.Sprintf("Value of type null returned from %s::__get() must be compatible with unset property %s::$%s of type %s",
								o.Class.GetName(), o.Class.GetName(), keyStr, hint.String()))
					}
					if !result.IsNull() && !hint.Check(ctx, result) {
						typeName := phpv.ZValTypeNameDetailed(result)
						return nil, ThrowError(ctx, TypeError,
							fmt.Sprintf("Value of type %s returned from %s::__get() must be compatible with unset property %s::$%s of type %s",
								typeName, o.Class.GetName(), o.Class.GetName(), keyStr, hint.String()))
					}
					// Coerce to matching type in weak mode
					if !ctx.Global().GetStrictTypes() && hint.Type() != phpv.ZtMixed && hint.Type() != phpv.ZtObject &&
						len(hint.Union) == 0 && len(hint.Intersection) == 0 && result.GetType() != hint.Type() {
						if coerced, err2 := result.As(ctx, hint.Type()); err2 == nil && coerced != nil {
							result = coerced.ZVal()
						}
					}
				}
			}
			return result, nil
		}
		// __get guard fired (recursion detected) - return null without warning
		// to match PHP behavior where recursive __get silently returns null
		return phpv.ZNULL.ZVal(), nil
	}

	// Emit "Undefined property" warning
	ctx.Warn("Undefined property: %s::$%s", o.GetClass().GetName(), keyStr, logopt.NoFuncName(true))

	return phpv.ZNULL.ZVal(), nil
}

// ObjectGetQuiet is like ObjectGet but returns (value, found, err) without emitting
// "Undefined property" warnings. Used for write-context auto-vivification paths.
func (o *ZObject) ObjectGetQuiet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, bool, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, false, err
	}

	keyStr := key.(phpv.ZString)

	// Check for property get hook (PHP 8.4) - only if not inside ANY hook for this property
	qInsideHook := (o.getHookGuard != nil && o.getHookGuard[keyStr]) ||
		(o.setHookGuard != nil && o.setHookGuard[keyStr])
	if !qInsideHook {
		if prop := o.findPropWithHook(keyStr); prop != nil {
			if prop.GetHook != nil {
				result, err := o.runGetHook(ctx, keyStr, prop)
				if err != nil {
					return nil, false, err
				}
				return result, true, nil
			}
			// Write-only virtual: set hook but no get hook, not backed, and no backing value
			if prop.SetHook != nil && prop.GetHook == nil && !prop.IsBacked && !o.h.HasString(keyStr) {
				return nil, false, ThrowError(ctx, Error,
					fmt.Sprintf("Property %s::$%s is write-only", o.Class.GetName(), keyStr))
			}
		}
	}

	if _, ok := o.hasPrivate[keyStr]; ok {
		resolveClass := o.resolvePrivateClass(ctx, keyStr)
		propName := getPrivatePropName(resolveClass, keyStr)
		if o.h.HasString(propName) {
			return o.h.GetString(propName), true, nil
		}
	}

	if o.h.HasString(keyStr) {
		return o.h.GetString(keyStr), true, nil
	}

	// Property not found; check for class-level property handler before __get
	if h := FindPropHandlers(o.Class); h != nil && h.HandlePropGet != nil {
		if result, err := h.HandlePropGet(ctx, o, keyStr); result != nil || err != nil {
			return result, err != nil, err
		}
	}

	// Property not found, try __get magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__get"]; ok {
		if o.getGuard == nil {
			o.getGuard = make(map[phpv.ZString]bool)
		}
		if !o.getGuard[keyStr] {
			o.getGuard[keyStr] = true
			result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
			delete(o.getGuard, keyStr)
			if err != nil {
				return nil, false, err
			}
			if result == nil {
				result = phpv.ZNULL.ZVal()
			}
			return result, true, nil
		}
	}

	return phpv.ZNULL.ZVal(), false, nil
}

func (o *ZObject) ObjectSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	keyStr := key.(phpv.ZString)

	// Lazy object: trigger initialization on property write (unless skipped)
	if o.IsLazy() && !o.IsPropertySkippedForLazy(keyStr) {
		if err := o.TriggerLazyInitForProp(ctx, keyStr); err != nil {
			return err
		}
		// After proxy init, delegate to the real instance
		if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil {
			target := o.ResolveProxy()
			if target.IsLazy() {
				if err := target.TriggerLazyInit(ctx); err != nil {
					return err
				}
				target = target.ResolveProxy()
			}
			return target.ObjectSet(ctx, keyStr, value)
		}
	} else if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil && !o.IsPropertySkippedForLazy(keyStr) {
		target := o.ResolveProxy()
		if target.IsLazy() {
			if err := target.TriggerLazyInit(ctx); err != nil {
				return err
			}
			target = target.ResolveProxy()
		}
		return target.ObjectSet(ctx, keyStr, value)
	}

	// PHP: property names starting with \0 are not allowed
	if len(keyStr) > 0 && keyStr[0] == 0 {
		return ThrowError(ctx, Error, "Cannot access property starting with \"\\0\"")
	}

	// Enum cases are immutable: properties cannot be written to or created
	if zc, ok := o.Class.(*ZClass); ok && zc.Type.Has(phpv.ZClassTypeEnum) {
		// Check if the property is a known enum property (name, value)
		if keyStr == "name" || (keyStr == "value" && zc.EnumBackingType != 0) {
			if value == nil {
				// unset() on readonly enum property
				return ThrowError(ctx, Error,
					fmt.Sprintf("Cannot unset readonly property %s::$%s", o.Class.GetName(), keyStr))
			}
			return ThrowError(ctx, Error,
				fmt.Sprintf("Cannot modify readonly property %s::$%s", o.Class.GetName(), keyStr))
		}
		if value == nil {
			// unset() on a non-existent property - still disallowed
			return ThrowError(ctx, Error,
				fmt.Sprintf("Cannot unset dynamic property %s::$%s", o.Class.GetName(), keyStr))
		}
		return ThrowError(ctx, Error,
			fmt.Sprintf("Cannot create dynamic property %s::$%s", o.Class.GetName(), keyStr))
	}

	// Readonly classes do not allow dynamic properties
	if zc, ok := o.Class.(*ZClass); ok && zc.Attr.Has(phpv.ZClassReadonly) && !o.h.HasString(keyStr) {
		hasDeclared := false
		for cur := zc; cur != nil; cur = cur.Extends {
			for _, p := range cur.Props {
				if p.VarName == keyStr {
					hasDeclared = true
					break
				}
			}
			if hasDeclared {
				break
			}
		}
		if !hasDeclared {
			if value == nil {
				return ThrowError(ctx, Error,
					fmt.Sprintf("Cannot create dynamic property %s::$%s", o.Class.GetName(), keyStr))
			}
			return ThrowError(ctx, Error,
				fmt.Sprintf("Cannot create dynamic property %s::$%s", o.Class.GetName(), keyStr))
		}
	}

	// Internal classes (e.g. Closure) that don't allow dynamic properties
	if zc, ok := o.Class.(*ZClass); ok && zc.InternalOnly && !o.h.HasString(keyStr) {
		hasDeclared := false
		for _, p := range zc.Props {
			if p.VarName == keyStr {
				hasDeclared = true
				break
			}
		}
		if !hasDeclared {
			return ThrowError(ctx, Error,
				fmt.Sprintf("Cannot create dynamic property %s::$%s", o.Class.GetName(), keyStr))
		}
	}

	// Check if accessing a static property as non-static
	o.checkStaticPropertyAccess(ctx, keyStr)

	// Check property visibility. If the property is not visible but __set/__unset exists,
	// PHP calls the magic method instead of throwing the visibility error.
	visErr := o.checkPropertyVisibility(ctx, keyStr, "access")
	if visErr != nil {
		if value == nil {
			// unset() on a non-visible property → try __unset
			class := o.GetClass().(*ZClass)
			if m, ok := class.Methods["__unset"]; ok {
				if o.unsetGuard == nil {
					o.unsetGuard = make(map[phpv.ZString]bool)
				}
				if !o.unsetGuard[keyStr] {
					o.unsetGuard[keyStr] = true
					_, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
					delete(o.unsetGuard, keyStr)
					return err
				}
			}
		} else {
			// set on a non-visible property → try __set
			class := o.GetClass().(*ZClass)
			if m, ok := class.Methods["__set"]; ok {
				if o.setGuard == nil {
					o.setGuard = make(map[phpv.ZString]bool)
				}
				if !o.setGuard[keyStr] {
					o.setGuard[keyStr] = true
					_, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal(), value}, o)
					delete(o.setGuard, keyStr)
					return err
				}
			}
		}
		return visErr
	}

	// Check asymmetric (set) visibility (PHP 8.4)
	if err := o.checkSetVisibility(ctx, keyStr, value == nil); err != nil {
		// When asymmetric visibility blocks and the property is UNSET,
		// fall back to __set/__unset magic methods.
		// If the property currently has a value, just throw the error.
		propInHash := o.h.HasString(keyStr) || (o.hasPrivate != nil && o.h.HasString(getPrivatePropName(o.Class.(*ZClass), keyStr)))
		// Property is considered "set" if it's in the hash table, OR if it's a typed
		// property that was never initialized (not explicitly unset).
		// Explicitly-unset typed properties allow __set/__unset fallback.
		propIsSet := propInHash
		if !propInHash {
			if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil {
				// Check if explicitly unset
				if o.typedPropUnset == nil || !(*o.typedPropUnset)[keyStr] {
					propIsSet = true // never initialized - treat as "set" for error purposes
				}
			}
		}
		if !propIsSet {
			class := o.GetClass().(*ZClass)
			if value == nil {
				if m, ok := class.Methods["__unset"]; ok {
					if o.unsetGuard == nil {
						o.unsetGuard = make(map[phpv.ZString]bool)
					}
					if !o.unsetGuard[keyStr] {
						o.unsetGuard[keyStr] = true
						_, err2 := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
						delete(o.unsetGuard, keyStr)
						return err2
					}
				}
			} else {
				if m, ok := class.Methods["__set"]; ok {
					if o.setGuard == nil {
						o.setGuard = make(map[phpv.ZString]bool)
					}
					if !o.setGuard[keyStr] {
						o.setGuard[keyStr] = true
						_, err2 := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal(), value}, o)
						delete(o.setGuard, keyStr)
						return err2
					}
				}
			}
		}
		return err
	}

	// Check readonly property enforcement
	if err := o.checkReadonlyWrite(ctx, keyStr); err != nil {
		return err
	}

	// PHP 8.4 property hooks: check for unset and read-only (get-only) virtual properties.
	// When inside EITHER hook for this property (get or set guard active), skip hook
	// dispatch and access the backing store directly.
	insideHook := (o.getHookGuard != nil && o.getHookGuard[keyStr]) ||
		(o.setHookGuard != nil && o.setHookGuard[keyStr])
	if !insideHook {
		if prop := o.findPropWithHook(keyStr); prop != nil {
			// Hooked properties cannot be unset
			if value == nil {
				return ThrowError(ctx, Error,
					fmt.Sprintf("Cannot unset hooked property %s::$%s", o.Class.GetName(), keyStr))
			}
			// Get-only virtual property: writing throws Error.
			// A property is virtual get-only if it has a get hook but no set hook
			// and no backing store. If the get hook references $this->prop, the property
			// has a backing store and allows implicit set (writing directly to backing).
			if prop.GetHook != nil && prop.SetHook == nil && prop.IsVirtual() {
				return ThrowError(ctx, Error,
					fmt.Sprintf("Property %s::$%s is read-only", o.Class.GetName(), keyStr))
			}
		}
	}

	// Enforce typed property type checking (PHP 8.0+)
	if value != nil {
		if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil {
			if coerced, err := o.enforcePropertyType(ctx, keyStr, prop, value); err != nil {
				return err
			} else if coerced != nil {
				value = coerced
			}
		}
	}

	// Check for property set hook (PHP 8.4) - only if not inside ANY hook for this property
	if !insideHook {
		if prop := o.findPropWithHook(keyStr); prop != nil && prop.SetHook != nil {
			return o.runSetHook(ctx, keyStr, prop, value)
		}
	}

	if _, ok := o.hasPrivate[keyStr]; ok {
		// Private properties are not virtual. If the caller's class declares a private
		// property with this name, resolve to the caller's copy, not the object's class copy.
		resolveClass := o.resolvePrivateClass(ctx, keyStr)
		propName := getPrivatePropName(resolveClass, keyStr)
		if o.h.HasString(propName) {
			return o.setPropRefTracked(propName, value)
		}
	}

	// Check if property exists in declared props OR is a declared typed property (uninitialized)
	propInHashTable := o.h.HasString(keyStr)
	isDeclaredTyped := false
	if !propInHashTable {
		if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil && !prop.Modifiers.IsPrivate() {
			isDeclaredTyped = true
		}
	}
	if propInHashTable || isDeclaredTyped {
		if value == nil {
			// Track that this typed property was explicitly unset
			if !propInHashTable {
				// Already not in hash table - nothing to unset from hash table
			}
			if prop := o.findDeclaredProp(keyStr); prop != nil && prop.TypeHint != nil {
				if o.typedPropUnset != nil {
					(*o.typedPropUnset)[keyStr] = true
				}
			}
			if propInHashTable {
				return o.h.SetString(keyStr, value) // removes from hash table
			}
			return nil
		}
		// Property is being set - clear the unset flag
		if o.typedPropUnset != nil {
			delete(*o.typedPropUnset, keyStr)
		}
		return o.setPropRefTracked(keyStr, value)
	}

	// Property not found; check for class-level property handler before magic methods
	if h := FindPropHandlers(o.Class); h != nil {
		if value == nil {
			if h.HandlePropUnset != nil {
				if handled, err := h.HandlePropUnset(ctx, o, keyStr); handled || err != nil {
					return err
				}
			}
		} else {
			if h.HandlePropSet != nil {
				if handled, err := h.HandlePropSet(ctx, o, keyStr, value); handled || err != nil {
					return err
				}
			}
		}
	}

	// Property not found, try magic methods
	class := o.GetClass().(*ZClass)
	if value == nil {
		// unset() on a non-existent property → try __unset
		if m, ok := class.Methods["__unset"]; ok {
			if o.unsetGuard == nil {
				o.unsetGuard = make(map[phpv.ZString]bool)
			}
			if !o.unsetGuard[keyStr] {
				o.unsetGuard[keyStr] = true
				_, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
				delete(o.unsetGuard, keyStr)
				return err
			}
		}
	} else {
		// set on a non-existent property → try __set
		if m, ok := class.Methods["__set"]; ok {
			if o.setGuard == nil {
				o.setGuard = make(map[phpv.ZString]bool)
			}
			if !o.setGuard[keyStr] {
				o.setGuard[keyStr] = true
				_, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal(), value}, o)
				delete(o.setGuard, keyStr)
				return err
			}
		}
	}

	// If the caller's own class declares a private property with this name and it was unset,
	// recreate it under the mangled name (PHP allows recreating unset private props
	// from within the declaring class). This check is after __set so that __set
	// takes priority when defined.
	if _, ok := o.hasPrivate[keyStr]; ok {
		if callerClass := ctx.Class(); callerClass != nil {
			if callerZClass, ok := callerClass.(*ZClass); ok {
				if prop, ok := getOwnProp(callerZClass, keyStr); ok && prop.Modifiers.IsPrivate() {
					propName := getPrivatePropName(callerClass, keyStr)
					return o.setPropRefTracked(propName, value)
				}
			}
		}
	}

	// Dynamic property creation deprecation (PHP 8.2+)
	// Only emit when creating a NEW property that is not declared in the class.
	// Don't warn for declared properties that were temporarily unset.
	// Don't warn if the class has __get or __set magic methods (implicit dynamic property support).
	// Private properties from parent classes are not visible to subclasses,
	// so creating a same-named property on the subclass IS a dynamic property creation.
	declaredProp := o.findDeclaredProp(keyStr)
	if declaredProp != nil && declaredProp.Modifiers.IsPrivate() {
		// Check if the caller's class is the declaring class
		callerClass := ctx.Class()
		declaringClass := o.findDeclaringClass(keyStr)
		if callerClass == nil || (declaringClass != nil && callerClass.GetName() != declaringClass.GetName()) {
			declaredProp = nil // treat as undeclared for deprecation purposes
		}
	}
	if value != nil && !o.allowsDynamicProperties() && declaredProp == nil {
		hasMagicProp := false
		if zc, ok := o.Class.(*ZClass); ok {
			_, hasGet := zc.Methods["__get"]
			_, hasSet := zc.Methods["__set"]
			hasMagicProp = hasGet || hasSet
		}
		if !hasMagicProp {
			ctx.Deprecated("Creation of dynamic property %s::$%s is deprecated",
				o.Class.GetName(), keyStr, logopt.NoFuncName(true))
		}
	}

	return o.setPropRefTracked(keyStr, value)
}

func (o *ZObject) NewIterator() phpv.ZIterator {
	// Lazy objects: foreach triggers initialization
	if o.IsLazy() {
		// Create a lazy-aware iterator that triggers init on first access
		return &lazyObjectIterator{obj: o, scope: nil}
	}
	if o.LazyState == LazyProxyInitialized && o.LazyInstance != nil {
		return o.LazyInstance.NewIterator()
	}
	return o.NewIteratorInScope(nil)
}

// classHasHooks checks if any class in the hierarchy has hooked properties.
func (o *ZObject) classHasHooks() bool {
	class := o.Class.(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.HasHooks {
				return true
			}
		}
	}
	return false
}

// NewIteratorInScope creates an iterator that respects property visibility
// relative to the given scope class. If scope is nil, only public properties
// are visible (external access). If scope matches the object's class or a
// parent, protected/private properties become visible accordingly.
// For lazy objects, this triggers initialization (e.g., for get_object_vars).
//
// For objects with hooked properties, this builds an ordered list of entries
// that includes virtual hooked properties (calling get hooks to produce values).
//
// Property keys in the hash table:
// - Public/Protected: bare "name"
// - Private: "*ClassName:name"
func (o *ZObject) NewIteratorInScope(scope phpv.ZClass) phpv.ZIterator {
	// If the class has hooked properties, use the hook-aware iterator
	if o.classHasHooks() {
		return newHookedObjectIterator(o, scope)
	}

	// Build set of protected property names to know which bare names are non-public
	protectedProps := make(map[phpv.ZString]struct{})
	class := o.Class.(*ZClass)
	for class != nil {
		for _, p := range class.Props {
			if p.Modifiers.IsProtected() {
				protectedProps[p.VarName] = struct{}{}
			}
		}
		parent := class.GetParent()
		if parent == nil {
			break
		}
		if pc, ok := parent.(*ZClass); ok && pc != nil {
			class = pc
		} else {
			break
		}
	}
	return &zobjectIterator{obj: o, inner: o.h.NewIterator(), scope: scope, protectedProps: protectedProps}
}

// hookedObjEntry represents a single property in the iteration order for
// objects with hooked properties.
type hookedObjEntry struct {
	key   phpv.ZString
	prop  *phpv.ZClassProp // non-nil for declared properties; nil for dynamic
	class *ZClass          // declaring class (for private property lookup)
}

// hookedObjectIterator iterates objects with property hooks in proper PHP order.
// It pre-computes the list of visible entries on construction so that the
// iteration order is stable and includes virtual hooked properties.
type hookedObjectIterator struct {
	obj     *ZObject
	entries []hookedObjEntry
	pos     int
}

// newHookedObjectIterator builds the iteration list for an object with hooks.
func newHookedObjectIterator(o *ZObject, scope phpv.ZClass) *hookedObjectIterator {
	it := &hookedObjectIterator{obj: o}

	// Build lineage from current class to root
	var lineage []*ZClass
	class := o.Class.(*ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		lineage = append(lineage, cur)
	}

	// Build most-derived map for non-private properties
	mostDerived := map[string]*phpv.ZClassProp{}
	mostDerivedClass := map[string]*ZClass{}
	for _, cl := range lineage {
		for _, p := range cl.Props {
			if p.Modifiers.IsStatic() {
				continue
			}
			if !p.Modifiers.IsPrivate() {
				if _, ok := mostDerived[p.VarName.String()]; !ok {
					mostDerived[p.VarName.String()] = p
					mostDerivedClass[p.VarName.String()] = cl
				}
			}
		}
	}

	// Collect declared properties in order (root to leaf)
	shown := map[string]struct{}{}
	for i := len(lineage) - 1; i >= 0; i-- {
		cl := lineage[i]
		for _, p := range cl.Props {
			if p.Modifiers.IsStatic() {
				continue
			}

			// Virtual set-only properties (no get hook): skip
			if p.IsVirtual() && !o.h.HasString(p.VarName) {
				if p.GetHook == nil {
					if !p.Modifiers.IsPrivate() {
						shown[p.VarName.String()] = struct{}{}
					}
					continue
				}
			}

			if !p.Modifiers.IsPrivate() {
				if _, ok := shown[p.VarName.String()]; ok {
					continue
				}
				// Use the most-derived version
				derived, hasDerived := mostDerived[p.VarName.String()]
				derivedClass := mostDerivedClass[p.VarName.String()]
				if hasDerived {
					p = derived
					cl = derivedClass
				}

				// Check visibility
				if !it.isVisibleNonPrivate(p, scope, class) {
					shown[p.VarName.String()] = struct{}{}
					continue
				}

				// Skip properties with no value and no get hook.
				// Properties with get hooks are always included (hook provides value).
				// Properties without get hooks must have a value in the hash table.
				if !o.h.HasString(p.VarName) && (p.GetHook == nil) {
					shown[p.VarName.String()] = struct{}{}
					continue
				}

				shown[p.VarName.String()] = struct{}{}
				it.entries = append(it.entries, hookedObjEntry{key: p.VarName, prop: p, class: cl})
			} else {
				// Private property - check visibility
				if scope == nil || scope.GetName() != cl.GetName() {
					continue
				}
				propName := getPrivatePropName(cl, p.VarName)
				if !p.HasHooks && !o.h.HasString(propName) && p.TypeHint == nil {
					continue
				}
				shown[p.VarName.String()] = struct{}{}
				it.entries = append(it.entries, hookedObjEntry{key: p.VarName, prop: p, class: cl})
			}
		}
	}

	// Add dynamic properties (in hash table but not declared)
	htIt := o.h.NewIterator()
	for htIt.Valid(nil) {
		k, _ := htIt.Key(nil)
		if k != nil {
			key := k.AsString(nil)
			// Skip private property keys (start with *)
			if len(key) > 0 && key[0] == '*' {
				htIt.Next(nil)
				continue
			}
			if _, ok := shown[string(key)]; !ok {
				it.entries = append(it.entries, hookedObjEntry{key: key})
			}
		}
		htIt.Next(nil)
	}

	return it
}

// isVisibleNonPrivate checks if a non-private property is visible from the given scope.
func (it *hookedObjectIterator) isVisibleNonPrivate(p *phpv.ZClassProp, scope phpv.ZClass, objClass *ZClass) bool {
	if p.Modifiers.IsPublic() {
		return true
	}
	if p.Modifiers.IsProtected() {
		if scope == nil {
			return false
		}
		// Check if scope is related to the object class
		return scope.InstanceOf(objClass) || objClass.InstanceOf(scope)
	}
	return true // default: visible
}

func (it *hookedObjectIterator) Valid(ctx phpv.Context) bool {
	return it.pos < len(it.entries)
}

func (it *hookedObjectIterator) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	if it.pos >= len(it.entries) {
		return nil, nil
	}
	e := it.entries[it.pos]
	return it.getValue(ctx, e)
}

func (it *hookedObjectIterator) getValue(ctx phpv.Context, e hookedObjEntry) (*phpv.ZVal, error) {
	if e.prop != nil {
		// Declared property - check for get hook
		if e.prop.HasHooks && e.prop.GetHook != nil {
			val, _, err := it.obj.GetPropValueOrHook(ctx, e.prop)
			if err != nil {
				return nil, err
			}
			if val != nil {
				return val, nil
			}
		}
		// Private property: look up with mangled name
		if e.prop.Modifiers.IsPrivate() && e.class != nil {
			propName := getPrivatePropName(e.class, e.key)
			if it.obj.h.HasString(propName) {
				return it.obj.h.GetString(propName), nil
			}
		}
	}
	// Regular property or dynamic property: look up in hash table
	if it.obj.h.HasString(e.key) {
		return it.obj.h.GetString(e.key), nil
	}
	return phpv.ZNULL.ZVal(), nil
}

func (it *hookedObjectIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	if it.pos >= len(it.entries) {
		return nil, nil
	}
	return it.entries[it.pos].key.ZVal(), nil
}

func (it *hookedObjectIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	it.pos++
	if it.pos >= len(it.entries) {
		return nil, nil
	}
	return it.getValue(ctx, it.entries[it.pos])
}

func (it *hookedObjectIterator) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	if it.pos > 0 {
		it.pos--
	}
	if it.pos < len(it.entries) {
		return it.getValue(ctx, it.entries[it.pos])
	}
	return nil, nil
}

func (it *hookedObjectIterator) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	it.pos = 0
	if len(it.entries) > 0 {
		return it.getValue(ctx, it.entries[0])
	}
	return nil, nil
}

func (it *hookedObjectIterator) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	if it.pos >= len(it.entries) {
		return it.Reset(ctx)
	}
	return nil, nil
}

func (it *hookedObjectIterator) End(ctx phpv.Context) (*phpv.ZVal, error) {
	if len(it.entries) > 0 {
		it.pos = len(it.entries) - 1
		return it.getValue(ctx, it.entries[it.pos])
	}
	return nil, nil
}

func (it *hookedObjectIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		for i, e := range it.entries {
			it.pos = i
			v, err := it.getValue(ctx, e)
			if err != nil {
				continue
			}
			if !yield(e.key.ZVal(), v) {
				return
			}
		}
	}
}

func (it *hookedObjectIterator) IterateRaw(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return it.Iterate(ctx)
}

func (it *hookedObjectIterator) CurrentMakeRef(ctx phpv.Context) (*phpv.ZVal, error) {
	if it.pos >= len(it.entries) {
		return nil, nil
	}
	e := it.entries[it.pos]

	// Check for readonly property
	if it.obj.IsReadonlyProperty(e.key) && it.obj.IsReadonlyPropertyInitialized(e.key) {
		return nil, ThrowError(nil, Error,
			fmt.Sprintf("Cannot acquire reference to readonly property %s::$%s", it.obj.GetClass().GetName(), e.key))
	}

	// For virtual hooked properties, cannot take a reference
	if e.prop != nil && e.prop.HasHooks && e.prop.GetHook != nil && e.prop.IsVirtual() && !it.obj.h.HasString(e.key) {
		return nil, ThrowError(ctx, Error,
			fmt.Sprintf("Cannot create reference to property %s::$%s", it.obj.GetClass().GetName(), e.key))
	}

	// For private properties, get from mangled name
	if e.prop != nil && e.prop.Modifiers.IsPrivate() && e.class != nil {
		propName := getPrivatePropName(e.class, e.key)
		if it.obj.h.HasString(propName) {
			return it.obj.h.GetString(propName), nil
		}
	}

	// Regular property: return reference from hash table
	if it.obj.h.HasString(e.key) {
		return it.obj.h.GetString(e.key), nil
	}
	return phpv.ZNULL.ZVal(), nil
}

func (it *hookedObjectIterator) CleanupRef() {
	// No cleanup needed for pre-computed entries
}

type zobjectIterator struct {
	obj            *ZObject
	inner          phpv.ZIterator
	scope          phpv.ZClass // nil means external access (only public)
	protectedProps map[phpv.ZString]struct{}
}

func (it *zobjectIterator) skipNonPublic(ctx phpv.Context) {
	for it.inner.Valid(ctx) {
		k, _ := it.inner.Key(ctx)
		if k == nil {
			break
		}
		key := k.AsString(ctx)

		// Check for private property format: *ClassName:propName
		if len(key) > 0 && key[0] == '*' {
			if it.scope != nil {
				// Extract class name from *ClassName:propName
				colonIdx := -1
				for i := 1; i < len(key); i++ {
					if key[i] == ':' {
						colonIdx = i
						break
					}
				}
				if colonIdx > 0 {
					className := key[1:colonIdx]
					if string(it.scope.GetName()) == string(className) {
						break // visible - scope matches declaring class
					}
				}
			}
			// Not visible, skip
			it.inner.Next(ctx)
			continue
		}

		// Bare key - check if it's a protected property
		if _, isProtected := it.protectedProps[key]; isProtected {
			if it.scope != nil {
				break // visible - we're inside a class method
			}
			it.inner.Next(ctx)
			continue
		}

		// Public property or dynamic property - always visible
		break
	}
}

func (it *zobjectIterator) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	it.skipNonPublic(ctx)
	return it.inner.Current(ctx)
}

func (it *zobjectIterator) CurrentMakeRef(ctx phpv.Context) (*phpv.ZVal, error) {
	it.skipNonPublic(ctx)

	// Check for readonly property — cannot acquire reference to readonly property
	if it.obj != nil {
		k, _ := it.inner.Key(ctx)
		if k != nil {
			keyStr := k.AsString(ctx)
			// Strip private property prefix (*ClassName:propName -> propName)
			if len(keyStr) > 0 && keyStr[0] == '*' {
				for i := 1; i < len(keyStr); i++ {
					if keyStr[i] == ':' {
						keyStr = keyStr[i+1:]
						break
					}
				}
			}
			if it.obj.IsReadonlyProperty(keyStr) && it.obj.IsReadonlyPropertyInitialized(keyStr) {
				return nil, ThrowError(ctx, Error,
					fmt.Sprintf("Cannot acquire reference to readonly property %s::$%s", it.obj.GetClass().GetName(), keyStr))
			}
		}
	}

	if inner, ok := it.inner.(interface {
		CurrentMakeRef(phpv.Context) (*phpv.ZVal, error)
	}); ok {
		return inner.CurrentMakeRef(ctx)
	}
	return it.inner.Current(ctx)
}

func (it *zobjectIterator) CleanupRef() {
	if ri, ok := it.inner.(interface{ CleanupRef() }); ok {
		ri.CleanupRef()
	}
}

func (it *zobjectIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	it.skipNonPublic(ctx)
	k, err := it.inner.Key(ctx)
	if err != nil || k == nil {
		return k, err
	}
	// Strip *ClassName: prefix from private property keys
	key := k.AsString(ctx)
	if len(key) > 0 && key[0] == '*' {
		for i := 1; i < len(key); i++ {
			if key[i] == ':' {
				return phpv.ZString(key[i+1:]).ZVal(), nil
			}
		}
	}
	return k, nil
}

func (it *zobjectIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	it.inner.Next(ctx)
	it.skipNonPublic(ctx)
	return it.inner.Current(ctx)
}

func (it *zobjectIterator) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	return it.inner.Prev(ctx)
}

func (it *zobjectIterator) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := it.inner.Reset(ctx)
	it.skipNonPublic(ctx)
	return v, err
}

func (it *zobjectIterator) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	return it.inner.ResetIfEnd(ctx)
}

func (it *zobjectIterator) End(ctx phpv.Context) (*phpv.ZVal, error) {
	return it.inner.End(ctx)
}

func (it *zobjectIterator) Valid(ctx phpv.Context) bool {
	it.skipNonPublic(ctx)
	return it.inner.Valid(ctx)
}

func (it *zobjectIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {
		for it.skipNonPublic(ctx); it.inner.Valid(ctx); it.inner.Next(ctx) {
			it.skipNonPublic(ctx)
			if !it.inner.Valid(ctx) {
				break
			}
			key, _ := it.inner.Key(ctx)
			value, _ := it.inner.Current(ctx)
			if !yield(key, value) {
				break
			}
		}
	}
}

func (it *zobjectIterator) IterateRaw(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	return it.Iterate(ctx)
}

func (a *ZObject) Count(ctx phpv.Context) phpv.ZInt {
	// Count non-static declared properties across the class hierarchy,
	// plus any dynamic properties set on the instance.
	// Uninitialized typed properties are NOT counted.
	count := 0
	for prop := range a.IterProps(ctx) {
		if prop.TypeHint != nil && !a.HasPropValue(prop) {
			continue // uninitialized typed property
		}
		count++
	}
	return phpv.ZInt(count)
}

func (a *ZObject) HashTable() *phpv.ZHashTable {
	return a.h
}

func (a *ZObject) GetClass() phpv.ZClass {
	if c, ok := a.CurrentClass.(*ZClass); ok && c != nil {
		return a.CurrentClass
	}
	return a.Class
}

func (a *ZObject) String() string {
	return "Object"
}

func (a *ZObject) Value() phpv.Val {
	return a
}

// resolvePrivateClass determines which class's private property to access.
// If the caller's class declares a private property with the given name,
// the caller's class is returned (private properties are not virtual).
// Otherwise, falls back to the object's runtime class.
func (o *ZObject) resolvePrivateClass(ctx phpv.Context, keyStr phpv.ZString) phpv.ZClass {
	callerClass := ctx.Class()
	if callerClass != nil {
		if callerZClass, ok := callerClass.(*ZClass); ok {
			// Only check the caller's OWN declared props (not walking hierarchy).
			// D extending C with private $p: D's methods should NOT resolve to D.
			if prop, ok := getOwnProp(callerZClass, keyStr); ok && prop.Modifiers.IsPrivate() && !prop.Modifiers.IsStatic() {
				return callerClass
			}
		}
	}
	return o.GetClass()
}

// checkStaticPropertyAccess checks if the named property is declared as static
// in the class hierarchy and emits a notice if the caller has access to it.
// For protected/private static properties that the caller cannot access, no
// notice is emitted (the visibility error from checkPropertyVisibility takes precedence).
func (o *ZObject) checkStaticPropertyAccess(ctx phpv.Context, keyStr phpv.ZString) bool {
	// If the caller's own class has a private non-static property with this name,
	// the private property takes precedence — don't emit the static notice.
	if callerClass := ctx.Class(); callerClass != nil {
		if callerZClass, ok := callerClass.(*ZClass); ok {
			if prop, ok := getOwnProp(callerZClass, keyStr); ok && !prop.Modifiers.IsStatic() && prop.Modifiers.IsPrivate() {
				return false
			}
		}
	}
	class := o.Class.(*ZClass)
	for class != nil {
		if prop, ok := getOwnProp(class, keyStr); ok && prop.Modifiers.IsStatic() {
			// Only emit notice if the caller has access to this property.
			// Properties without explicit access modifier (access=0) are implicitly public.
			access := prop.Modifiers.Access()
			if access == phpv.ZAttrPublic || access == 0 {
				ctx.Notice("Accessing static property %s::$%s as non static", o.Class.GetName(), keyStr, logopt.NoFuncName(true))
			} else {
				callerClass := ctx.Class()
				if callerClass != nil {
					if prop.Modifiers.IsProtected() {
						if callerClass.InstanceOf(class) || class.InstanceOf(callerClass) {
							ctx.Notice("Accessing static property %s::$%s as non static", o.Class.GetName(), keyStr, logopt.NoFuncName(true))
						}
					} else if prop.Modifiers.IsPrivate() {
						// Compare with the declaring class (where the property is actually defined)
						if callerClass.GetName() == class.GetName() {
							ctx.Notice("Accessing static property %s::$%s as non static", o.Class.GetName(), keyStr, logopt.NoFuncName(true))
						}
					}
				}
			}
			return true
		}
		parent := class.GetParent()
		if parent == nil {
			break
		}
		class = parent.(*ZClass)
	}
	return false
}

func getPrivatePropName(class phpv.ZClass, name phpv.ZString) phpv.ZString {
	return phpv.ZString(fmt.Sprintf("*%s:%s", class.GetName(), name))
}

// GetPrivatePropNameExt is the exported version of getPrivatePropName for use
// by other packages (e.g., the reflection extension).
func GetPrivatePropNameExt(class phpv.ZClass, name phpv.ZString) phpv.ZString {
	return getPrivatePropName(class, name)
}

// getOwnProp checks only this class's directly declared Props (NOT walking
// the hierarchy via GetProp). Returns the prop and true if found.
func getOwnProp(class *ZClass, name phpv.ZString) (*phpv.ZClassProp, bool) {
	for _, p := range class.Props {
		if p.VarName == name {
			return p, true
		}
	}
	return nil, false
}

// findDeclaredProp walks the class hierarchy to find a declared property by name.
// FindDeclaredProp looks up a declared class property by name.
func (o *ZObject) FindDeclaredProp(keyStr phpv.ZString) *phpv.ZClassProp {
	return o.findDeclaredProp(keyStr)
}

func (o *ZObject) findDeclaredProp(keyStr phpv.ZString) *phpv.ZClassProp {
	class, ok := o.Class.(*ZClass)
	if !ok {
		return nil
	}
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == keyStr {
				return prop
			}
		}
	}
	return nil
}

// findDeclaringClass returns the ZClass that declares the given property.
func (o *ZObject) findDeclaringClass(keyStr phpv.ZString) *ZClass {
	class, ok := o.Class.(*ZClass)
	if !ok {
		return nil
	}
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName == keyStr {
				return cur
			}
		}
	}
	return nil
}

// enforcePropertyType checks that a value is compatible with a typed property's type hint.
// Returns a coerced value if coercion is needed and possible, or an error if the type is incompatible.
// In strict_types=1 mode, no coercion is performed (except int->float widening).
// EnforcePropertyType is the exported wrapper for enforcePropertyType,
// used by the reflection extension to validate types before direct property writes.
func (o *ZObject) EnforcePropertyType(ctx phpv.Context, keyStr phpv.ZString, prop *phpv.ZClassProp, value *phpv.ZVal) (*phpv.ZVal, error) {
	return o.enforcePropertyType(ctx, keyStr, prop, value)
}

func (o *ZObject) enforcePropertyType(ctx phpv.Context, keyStr phpv.ZString, prop *phpv.ZClassProp, value *phpv.ZVal) (*phpv.ZVal, error) {
	hint := prop.TypeHint
	if hint == nil {
		return nil, nil
	}

	// Use the declaring class name for error messages (not the runtime class)
	declClassName := o.Class.GetName()
	if dc := o.findDeclaringClass(keyStr); dc != nil {
		declClassName = dc.GetName()
	}

	// Null check
	if value.IsNull() {
		if hint.IsNullable() {
			return nil, nil
		}
		return nil, ThrowError(ctx, TypeError,
			fmt.Sprintf("Cannot assign null to property %s::$%s of type %s",
				declClassName, keyStr, hint.String()))
	}

	// strict_types applies to the file where the assignment happens
	isStrict := ctx.Global().GetStrictTypes()

	if isStrict {
		// In strict mode, use strict type checking (no coercion except int->float)
		if hint.CheckStrict(ctx, value) {
			// Even in strict mode, int values assigned to float properties get widened
			hintType := hint.Type()
			if hintType == phpv.ZtFloat && value.GetType() == phpv.ZtInt && len(hint.Union) == 0 && len(hint.Intersection) == 0 {
				if coerced, err := value.Value().AsVal(ctx, phpv.ZtFloat); err == nil && coerced != nil {
					return coerced.ZVal(), nil
				}
			}
			return nil, nil
		}
		// Strict type mismatch
		typeName := phpv.ZValTypeNameDetailed(value)
		return nil, ThrowError(ctx, TypeError,
			fmt.Sprintf("Cannot assign %s to property %s::$%s of type %s",
				typeName, declClassName, keyStr, hint.String()))
	}

	// Weak mode: check if value matches the type hint
	if hint.Check(ctx, value) {
		// For scalar types, coerce the value to the exact type
		hintType := hint.Type()
		valType := value.GetType()
		if hintType != phpv.ZtMixed && hintType != phpv.ZtObject && valType != hintType && len(hint.Union) == 0 && len(hint.Intersection) == 0 {
			// Emit implicit conversion deprecation for float->int
			if hintType == phpv.ZtInt && valType == phpv.ZtFloat {
				v, err := phpv.FloatToIntImplicit(ctx, value.Value().(phpv.ZFloat))
				if err != nil {
					return nil, err
				}
				return v.ZVal(), nil
			}
			if coerced, err := value.Value().AsVal(ctx, hintType); err == nil && coerced != nil {
				return coerced.ZVal(), nil
			}
		}
		return nil, nil
	}

	// Type mismatch - throw TypeError
	typeName := phpv.ZValTypeNameDetailed(value)
	return nil, ThrowError(ctx, TypeError,
		fmt.Sprintf("Cannot assign %s to property %s::$%s of type %s",
			typeName, declClassName, keyStr, hint.String()))
}

// allowsDynamicProperties checks if the object's class allows dynamic property creation.
// stdClass, classes with #[AllowDynamicProperties], and their descendants are exempt.
// AllowsDynamicProperties returns true if the object's class allows dynamic property creation.
func (o *ZObject) AllowsDynamicProperties() bool {
	return o.allowsDynamicProperties()
}

func (o *ZObject) allowsDynamicProperties() bool {
	class, ok := o.Class.(*ZClass)
	if !ok {
		return true // non-ZClass implementations allow dynamic props
	}
	// Walk the class hierarchy
	for cur := class; cur != nil; cur = cur.Extends {
		name := cur.Name
		// stdClass and __PHP_Incomplete_Class allow dynamic properties
		if name == "stdClass" || name == "__PHP_Incomplete_Class" {
			return true
		}
		// Check for #[AllowDynamicProperties] attribute
		for _, attr := range cur.Attributes {
			if attr.ClassName == "AllowDynamicProperties" || attr.ClassName == "\\AllowDynamicProperties" {
				return true
			}
		}
	}
	return false
}

package phpobj

import (
	"fmt"
	"iter"

	"github.com/MagicalTux/goro/core/phpv"
)

// Lazy object state constants
const (
	LazyNone             = 0 // Not a lazy object
	LazyGhostUninitialized = 1 // Lazy ghost, not yet initialized
	LazyProxyUninitialized = 2 // Lazy proxy, not yet initialized
	LazyGhostInitialized   = 3 // Lazy ghost, initialized
	LazyProxyInitialized   = 4 // Lazy proxy, initialized
)

// IsLazy returns true if the object is a lazy object (ghost or proxy) that has
// not yet been initialized.
func (o *ZObject) IsLazy() bool {
	return o.LazyState == LazyGhostUninitialized || o.LazyState == LazyProxyUninitialized
}

// IsLazyGhost returns true if the object is a lazy ghost (initialized or not).
func (o *ZObject) IsLazyGhost() bool {
	return o.LazyState == LazyGhostUninitialized || o.LazyState == LazyGhostInitialized
}

// IsLazyProxy returns true if the object is a lazy proxy (initialized or not).
func (o *ZObject) IsLazyProxy() bool {
	return o.LazyState == LazyProxyUninitialized || o.LazyState == LazyProxyInitialized
}

// IsLazyInitialized returns true if the object is a lazy object that has been
// initialized (either ghost or proxy).
func (o *ZObject) IsLazyInitialized() bool {
	return o.LazyState == LazyGhostInitialized || o.LazyState == LazyProxyInitialized
}

// MakeLazyGhost sets up this object as a lazy ghost with the given initializer.
// The object's non-skipped properties are cleared (typed properties become
// uninitialized, untyped properties are removed).
func (o *ZObject) MakeLazyGhost(initializer *phpv.ZVal) {
	o.LazyState = LazyGhostUninitialized
	o.LazyInitializer = initializer
	o.LazyInstance = nil
	o.LazySkippedProps = nil
	o.LazyInitializing = false

	// Clear all property values - lazy objects start with no properties
	o.h = phpv.NewHashTable()

	// If the class has no non-static, non-virtual properties, auto-realize immediately
	o.checkAutoRealizeNoProps()
}

// MakeLazyProxy sets up this object as a lazy proxy with the given factory.
func (o *ZObject) MakeLazyProxy(factory *phpv.ZVal) {
	o.LazyState = LazyProxyUninitialized
	o.LazyInitializer = factory
	o.LazyInstance = nil
	o.LazySkippedProps = nil
	o.LazyInitializing = false

	// Clear all property values
	o.h = phpv.NewHashTable()

	// If the class has no non-static, non-virtual properties, auto-realize immediately
	o.checkAutoRealizeNoProps()
}

// checkAutoRealizeNoProps checks if the class has zero non-static, non-virtual
// properties, and if so marks the lazy object as initialized immediately.
func (o *ZObject) checkAutoRealizeNoProps() {
	if !o.IsLazy() {
		return
	}
	zc, ok := o.Class.(*ZClass)
	if !ok {
		// Non-ZClass (e.g. stdClass) has no declared properties
		o.LazyState = LazyGhostInitialized
		o.LazyInitializer = nil
		return
	}

	for cur := zc; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.Modifiers.IsStatic() || p.IsVirtual() {
				continue
			}
			return // Has at least one non-static, non-virtual property
		}
	}

	// No non-static, non-virtual properties - realize immediately
	if o.LazyState == LazyGhostUninitialized {
		o.LazyState = LazyGhostInitialized
	} else if o.LazyState == LazyProxyUninitialized {
		o.LazyState = LazyProxyInitialized
	}
	o.LazyInitializer = nil
}

// IsPropertySkippedForLazy checks if a property has been marked as "skipped"
// for lazy initialization purposes.
func (o *ZObject) IsPropertySkippedForLazy(propName phpv.ZString) bool {
	if o.LazySkippedProps == nil {
		return false
	}
	return o.LazySkippedProps[propName]
}

// SkipLazyProperty marks a property as skipped for lazy initialization.
// When a property is skipped, accessing it does not trigger initialization
// and it gets its default value.
func (o *ZObject) SkipLazyProperty(ctx phpv.Context, propName phpv.ZString) {
	if o.LazySkippedProps == nil {
		o.LazySkippedProps = make(map[phpv.ZString]bool)
	}
	o.LazySkippedProps[propName] = true

	// Set the default value for this property
	if zc, ok := o.Class.(*ZClass); ok {
		for cur := zc; cur != nil; cur = cur.Extends {
			for _, p := range cur.Props {
				if p.VarName == propName && !p.Modifiers.IsStatic() {
					if p.Default != nil {
						val := dupDefault(p.Default)
						if p.Modifiers.IsPrivate() {
							k := getPrivatePropName(cur, p.VarName)
							o.h.SetString(k, val)
						} else {
							o.h.SetString(p.VarName, val)
						}
					} else if p.TypeHint == nil {
						// Untyped property without default gets null
						if p.Modifiers.IsPrivate() {
							k := getPrivatePropName(cur, p.VarName)
							o.h.SetString(k, phpv.ZNULL.ZVal())
						} else {
							o.h.SetString(p.VarName, phpv.ZNULL.ZVal())
						}
					}
					// Typed properties without default remain uninitialized
					break
				}
			}
		}
	}

	// Check if all non-static, non-virtual properties are now skipped.
	// If so, auto-realize the lazy object.
	o.checkAutoRealize(ctx)
}

// checkAutoRealize checks if all non-static, non-virtual properties have been
// skipped, and if so, marks the object as initialized (realized).
func (o *ZObject) checkAutoRealize(ctx phpv.Context) {
	if !o.IsLazy() {
		return
	}
	zc, ok := o.Class.(*ZClass)
	if !ok {
		return
	}

	for cur := zc; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.Modifiers.IsStatic() || p.IsVirtual() {
				continue
			}
			if !o.IsPropertySkippedForLazy(p.VarName) {
				return // Not all properties are skipped
			}
		}
	}

	// All properties are skipped - realize the object
	if o.LazyState == LazyGhostUninitialized {
		o.LazyState = LazyGhostInitialized
	} else if o.LazyState == LazyProxyUninitialized {
		o.LazyState = LazyProxyInitialized
	}
	o.LazyInitializer = nil
}

// SetRawValueWithoutLazyInit sets a property value on a lazy object without
// triggering initialization.
func (o *ZObject) SetRawValueWithoutLazyInit(ctx phpv.Context, propName phpv.ZString, value *phpv.ZVal) {
	if o.LazySkippedProps == nil {
		o.LazySkippedProps = make(map[phpv.ZString]bool)
	}
	o.LazySkippedProps[propName] = true

	// Find the property in the class hierarchy to handle private properly
	if zc, ok := o.Class.(*ZClass); ok {
		for cur := zc; cur != nil; cur = cur.Extends {
			for _, p := range cur.Props {
				if p.VarName == propName && !p.Modifiers.IsStatic() {
					if p.Modifiers.IsPrivate() {
						k := getPrivatePropName(cur, p.VarName)
						o.h.SetString(k, value)
					} else {
						o.h.SetString(p.VarName, value)
					}
					// Check auto-realize after setting
					o.checkAutoRealize(ctx)
					return
				}
			}
		}
	}

	// Dynamic property or fallback
	o.h.SetString(propName, value)
	o.checkAutoRealize(ctx)
}

// TriggerLazyInit triggers lazy initialization if the object is lazy and
// the accessed property is not skipped. Returns true if initialization happened.
func (o *ZObject) TriggerLazyInit(ctx phpv.Context) error {
	if !o.IsLazy() {
		return nil
	}
	if o.LazyInitializing {
		return nil
	}

	return o.doLazyInit(ctx)
}

// TriggerLazyInitForProp triggers lazy initialization if the object is lazy
// and the given property is not skipped. Returns nil if property is skipped
// or initialization succeeded.
func (o *ZObject) TriggerLazyInitForProp(ctx phpv.Context, propName phpv.ZString) error {
	if !o.IsLazy() {
		return nil
	}
	if o.IsPropertySkippedForLazy(propName) {
		return nil
	}
	if o.LazyInitializing {
		return nil
	}

	return o.doLazyInit(ctx)
}

// doLazyInit performs the actual lazy initialization.
func (o *ZObject) doLazyInit(ctx phpv.Context) error {
	o.LazyInitializing = true
	defer func() { o.LazyInitializing = false }()

	if o.LazyState == LazyGhostUninitialized {
		return o.doGhostInit(ctx)
	} else if o.LazyState == LazyProxyUninitialized {
		return o.doProxyInit(ctx)
	}
	return nil
}

// doGhostInit runs the ghost initializer.
func (o *ZObject) doGhostInit(ctx phpv.Context) error {
	// Save current state for rollback on exception
	savedH := o.h.Dup()
	savedState := o.LazyState

	// Initialize properties to their default values BEFORE calling the initializer.
	// The initializer sees the object with default values already set.
	o.initDefaultProps(ctx)

	// Mark as initialized before calling the initializer so that property
	// accesses inside the initializer don't trigger recursive initialization.
	o.LazyState = LazyGhostInitialized

	// Resolve the initializer ZVal to a Callable
	callable, resolveErr := FiberResolveCallable(ctx, o.LazyInitializer)
	if resolveErr != nil {
		// Rollback: restore saved state, object remains lazy
		o.h = savedH
		o.LazyState = savedState
		return resolveErr
	}

	// Call the initializer: $initializer($obj)
	result, err := ctx.CallZVal(ctx, callable, []*phpv.ZVal{o.ZVal()})
	if err != nil {
		// Rollback: restore saved state, object remains lazy
		o.h = savedH
		o.LazyState = savedState
		return err
	}

	// Ghost initializer must return null or void
	if result != nil && !result.IsNull() {
		// Rollback
		o.h = savedH
		o.LazyState = savedState
		return ThrowError(ctx, TypeError, "Lazy object initializer must return NULL or no value")
	}

	// Mark as initialized
	o.LazyInitializer = nil

	return nil
}

// doProxyInit runs the proxy factory.
func (o *ZObject) doProxyInit(ctx phpv.Context) error {
	// Resolve the factory ZVal to a Callable
	callable, resolveErr := FiberResolveCallable(ctx, o.LazyInitializer)
	if resolveErr != nil {
		return resolveErr
	}

	// Call the factory: $factory($obj)
	result, err := ctx.CallZVal(ctx, callable, []*phpv.ZVal{o.ZVal()})
	if err != nil {
		return err
	}

	// Factory must return an object
	if result == nil || result.GetType() != phpv.ZtObject {
		className := o.Class.GetName()
		return ThrowError(ctx, TypeError,
			fmt.Sprintf("Lazy proxy factory must return an instance of a class compatible with %s, null returned", className))
	}

	realObj, ok := result.Value().(*ZObject)
	if !ok {
		className := o.Class.GetName()
		return ThrowError(ctx, TypeError,
			fmt.Sprintf("Lazy proxy factory must return an instance of a class compatible with %s, null returned", className))
	}

	// Cannot return itself
	if realObj == o {
		return ThrowError(ctx, Error, "Lazy proxy factory must return a non-lazy object")
	}

	// Cannot return another lazy object
	if realObj.IsLazy() {
		return ThrowError(ctx, Error, "Lazy proxy factory must return a non-lazy object")
	}

	// Validate compatibility: the real instance must be the same class or a parent class
	if err := validateProxyClass(o, realObj); err != nil {
		return ThrowError(ctx, TypeError, err.Error())
	}

	// Store the real instance
	o.LazyInstance = realObj
	o.LazyState = LazyProxyInitialized
	o.LazyInitializer = nil

	return nil
}

// validateProxyClass checks that the real instance is compatible with the proxy.
// The real instance must be the same class or a parent class.
// The proxy class must not add any additional properties or override __destruct/__clone.
func validateProxyClass(proxy *ZObject, real *ZObject) error {
	proxyClass := proxy.Class
	realClass := real.Class

	// Same class is always OK
	if proxyClass.GetName() == realClass.GetName() {
		return nil
	}

	// The proxy class must be the same as or a subclass of the real instance's class
	if !proxyClass.InstanceOf(realClass) {
		return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
			"The proxy must be a instance of the same class as the real instance, or a sub-class "+
			"with no additional properties, and no overrides of the __destructor or __clone methods.",
			realClass.GetName(), proxyClass.GetName())
	}

	// Check no additional properties in the proxy class hierarchy between proxy and real
	proxyZc, ok1 := proxyClass.(*ZClass)
	realZc, ok2 := realClass.(*ZClass)
	if ok1 && ok2 {
		// Check for additional properties
		for cur := proxyZc; cur != nil && cur != realZc; cur = cur.Extends {
			for _, p := range cur.Props {
				if !p.Modifiers.IsStatic() {
					// Check if this property exists in realClass
					found := false
					for rcur := realZc; rcur != nil; rcur = rcur.Extends {
						for _, rp := range rcur.Props {
							if rp.VarName == p.VarName {
								found = true
								break
							}
						}
						if found {
							break
						}
					}
					if !found {
						return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
							"The proxy must be a instance of the same class as the real instance, or a sub-class "+
							"with no additional properties, and no overrides of the __destructor or __clone methods.",
							realClass.GetName(), proxyClass.GetName())
					}
				}
			}
		}

		// Check for __destruct override
		if m, ok := proxyZc.Methods["__destruct"]; ok {
			if realM, ok2 := realZc.Methods["__destruct"]; ok2 {
				if m != realM {
					return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
						"The proxy must be a instance of the same class as the real instance, or a sub-class "+
						"with no additional properties, and no overrides of the __destructor or __clone methods.",
						realClass.GetName(), proxyClass.GetName())
				}
			} else {
				return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
					"The proxy must be a instance of the same class as the real instance, or a sub-class "+
					"with no additional properties, and no overrides of the __destructor or __clone methods.",
					realClass.GetName(), proxyClass.GetName())
			}
		}

		// Check for __clone override
		if m, ok := proxyZc.Methods["__clone"]; ok {
			if realM, ok2 := realZc.Methods["__clone"]; ok2 {
				if m != realM {
					return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
						"The proxy must be a instance of the same class as the real instance, or a sub-class "+
						"with no additional properties, and no overrides of the __destructor or __clone methods.",
						realClass.GetName(), proxyClass.GetName())
				}
			} else {
				return fmt.Errorf("The real instance class %s is not compatible with the proxy class %s. "+
					"The proxy must be a instance of the same class as the real instance, or a sub-class "+
					"with no additional properties, and no overrides of the __destructor or __clone methods.",
					realClass.GetName(), proxyClass.GetName())
			}
		}
	}

	return nil
}

// initDefaultProps initializes properties to their default values for any
// property that was not set by the initializer.
func (o *ZObject) initDefaultProps(ctx phpv.Context) {
	zc, ok := o.Class.(*ZClass)
	if !ok {
		return
	}

	for cur := zc; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.Modifiers.IsStatic() {
				continue
			}
			if p.IsVirtual() {
				continue
			}

			if p.Modifiers.IsPrivate() {
				k := getPrivatePropName(cur, p.VarName)
				if !o.h.HasString(k) {
					if p.Default != nil {
						o.h.SetString(k, dupDefault(p.Default))
					} else if p.TypeHint == nil {
						o.h.SetString(k, phpv.ZNULL.ZVal())
					}
				}
			} else {
				if !o.h.HasString(p.VarName) {
					if p.Default != nil {
						o.h.SetString(p.VarName, dupDefault(p.Default))
					} else if p.TypeHint == nil {
						o.h.SetString(p.VarName, phpv.ZNULL.ZVal())
					}
				}
			}
		}
	}
}

// MarkLazyAsInitialized marks a lazy object as initialized without calling
// the initializer. Properties get their default values.
func (o *ZObject) MarkLazyAsInitialized(ctx phpv.Context) {
	if !o.IsLazy() {
		return
	}

	if o.LazyState == LazyGhostUninitialized {
		o.LazyState = LazyGhostInitialized
	} else if o.LazyState == LazyProxyUninitialized {
		o.LazyState = LazyProxyInitialized
	}
	o.LazyInitializer = nil

	// Initialize all properties to defaults
	o.initDefaultProps(ctx)
}

// GetProxyInstance returns the real instance for an initialized proxy, or nil.
// For a lazy proxy that has a nested proxy chain, this resolves through the chain.
func (o *ZObject) GetProxyInstance() *ZObject {
	if o.LazyState != LazyProxyInitialized {
		return nil
	}
	return o.LazyInstance
}

// ResolveProxy resolves the proxy chain, returning the innermost real object.
// For initialized proxies, property accesses are delegated to the real instance.
func (o *ZObject) ResolveProxy() *ZObject {
	if o.LazyInstance == nil {
		return o
	}
	// Follow proxy chain
	cur := o.LazyInstance
	for cur.LazyInstance != nil && cur.LazyState == LazyProxyInitialized {
		cur = cur.LazyInstance
	}
	// If the resolved instance is itself a lazy proxy that needs initialization,
	// return it so the caller can trigger init
	return cur
}

// lazyObjectIterator wraps an iterator that triggers lazy initialization on
// first use (e.g., foreach on a lazy object).
type lazyObjectIterator struct {
	obj     *ZObject
	scope   phpv.ZClass
	inner   phpv.ZIterator
	initErr error
}

func (it *lazyObjectIterator) ensureInit(ctx phpv.Context) {
	if it.inner != nil || it.initErr != nil {
		return
	}
	if it.obj.IsLazy() {
		it.initErr = it.obj.TriggerLazyInit(ctx)
		if it.initErr != nil {
			return
		}
	}
	// After init, delegate to the real object's iterator
	target := it.obj
	if target.LazyState == LazyProxyInitialized && target.LazyInstance != nil {
		target = target.ResolveProxy()
	}
	it.inner = target.NewIteratorInScope(it.scope)
}

func (it *lazyObjectIterator) Current(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.Current(ctx)
}

func (it *lazyObjectIterator) Key(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.Key(ctx)
}

func (it *lazyObjectIterator) Next(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.Next(ctx)
}

func (it *lazyObjectIterator) Prev(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.Prev(ctx)
}

func (it *lazyObjectIterator) Reset(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.Reset(ctx)
}

func (it *lazyObjectIterator) ResetIfEnd(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.ResetIfEnd(ctx)
}

func (it *lazyObjectIterator) End(ctx phpv.Context) (*phpv.ZVal, error) {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return nil, it.initErr
	}
	if it.inner == nil {
		return nil, nil
	}
	return it.inner.End(ctx)
}

func (it *lazyObjectIterator) Valid(ctx phpv.Context) bool {
	it.ensureInit(ctx)
	if it.initErr != nil {
		return false
	}
	if it.inner == nil {
		return false
	}
	return it.inner.Valid(ctx)
}

func (it *lazyObjectIterator) Iterate(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	it.ensureInit(ctx)
	if it.initErr != nil || it.inner == nil {
		return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {}
	}
	return it.inner.Iterate(ctx)
}

func (it *lazyObjectIterator) IterateRaw(ctx phpv.Context) iter.Seq2[*phpv.ZVal, *phpv.ZVal] {
	it.ensureInit(ctx)
	if it.initErr != nil || it.inner == nil {
		return func(yield func(*phpv.ZVal, *phpv.ZVal) bool) {}
	}
	return it.inner.IterateRaw(ctx)
}

// Err returns any pending error from lazy initialization.
func (it *lazyObjectIterator) Err() error {
	return it.initErr
}

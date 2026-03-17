package phpctx

import (
	"sync"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// Pool for FuncContext to reduce allocations during function calls
var funcContextPool = sync.Pool{
	New: func() any {
		return &FuncContext{
			h: phpv.NewHashTable(),
		}
	},
}

// GetFuncContext retrieves a FuncContext from the pool
func GetFuncContext() *FuncContext {
	return funcContextPool.Get().(*FuncContext)
}

// Release returns the FuncContext to the pool after clearing it.
// It returns an error if any destructor triggered during scope cleanup
// throws an exception.
func (c *FuncContext) Release() error {
	// Clean up foreach-by-reference iterators. In PHP, when the loop
	// variable goes out of scope (function return), the refcount on the
	// last iterated element drops to 1 and the reference wrapper is
	// removed. Since Goro has no refcounting, we call CleanupRef() on
	// each registered iterator to unwrap the last element's reference.
	for _, cleanup := range c.foreachRefCleanups {
		cleanup()
	}
	c.foreachRefCleanups = c.foreachRefCleanups[:0]

	// DecRef all objects in local variables before clearing the scope.
	// This ensures reference counts are properly decremented when leaving
	// function scope. We use DecRefImplicit because scope exit in PHP
	// allows destructors to run regardless of visibility.
	var releaseErr error
	if c.Context != nil {
		// Restore the global location to this function's call site before
		// running scope-exit destructors. In PHP, if a destructor triggered
		// during scope cleanup creates an exception, the stack trace should
		// show the original call site (e.g., the line that created the
		// outer object), not the last-executed line inside the function.
		if c.loc != nil {
			c.Context.Tick(c.Context, c.loc)
		}
		it := c.h.NewIterator()
		for it.Valid(c.Context) {
			v, err := it.Current(c.Context)
			if err == nil && v != nil && v.GetType() == phpv.ZtObject {
				if zobj, ok := v.Value().(phpv.ZObject); ok {
					// Call HandleDecRef if the class defines it (e.g. Closure releasing captured $this)
					if cls := zobj.GetClass(); cls != nil {
						if h := cls.Handlers(); h != nil && h.HandleDecRef != nil {
							h.HandleDecRef(c.Context, zobj)
						}
					}
				}
				if obj, ok := v.Value().(interface {
					DecRefImplicit(phpv.Context) error
				}); ok {
					if derr := obj.DecRefImplicit(c.Context); derr != nil {
						// Collect the last destructor error; in PHP, nested
						// destructor exceptions are chained via "previous".
						releaseErr = derr
					}
				}
			}
			it.Next(c.Context)
		}
	}

	c.Context = nil
	c.h.Empty()
	c.this = nil
	c.Args = c.Args[:0]
	c.c = nil
	c.loc = nil
	c.class = nil
	c.calledClass = nil
	c.methodType = ""
	c.isInternal = false
	funcContextPool.Put(c)
	return releaseErr
}

type FuncContext struct {
	phpv.Context

	h    *phpv.ZHashTable
	this phpv.ZObject
	Args []*phpv.ZVal
	c    phpv.Callable // called object (this function itself)

	loc *phpv.Loc

	class       phpv.ZClass
	calledClass phpv.ZClass // for late static binding (static::class)
	methodType  string

	isInternal bool // true when called from internal code (e.g., output buffer callbacks)

	foreachRefCleanups []func() // cleanup functions for foreach-by-reference iterators
}

// RegisterForeachRefCleanup registers a cleanup function to be called when
// this function context is released, for cleaning up foreach-by-reference
// iterator references.
func (c *FuncContext) RegisterForeachRefCleanup(fn func()) {
	c.foreachRefCleanups = append(c.foreachRefCleanups, fn)
}

func (c *FuncContext) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	a := c.h.Array()
	return a.AsVal(ctx, t)
}

func (c *FuncContext) GetType() phpv.ZType {
	return phpv.ZtArray
}

func (c *FuncContext) ZVal() *phpv.ZVal {
	return c.ZVal().Ref()
}

func (c *FuncContext) Func() phpv.FuncContext {
	return c
}

func (c *FuncContext) Callable() phpv.Callable {
	return c.c
}

func (c *FuncContext) This() phpv.ZObject {
	return c.this
}

func (c *FuncContext) Class() phpv.ZClass {
	return c.class
}

func (c *FuncContext) CalledClass() phpv.ZClass {
	if c.calledClass != nil {
		return c.calledClass
	}
	return c.class
}

func (c *FuncContext) OffsetExists(ctx phpv.Context, name phpv.Val) (bool, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return false, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		if c.this == nil {
			return false, nil
		}
		return true, nil
	case "GLOBALS":
		return true, nil
	case "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetExists(ctx, nameStr)
	}
	return c.h.HasString(nameStr), nil
}

func (c *FuncContext) OffsetGet(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		if c.this == nil {
			return nil, nil
		}
		return c.this.ZVal(), nil
	case "GLOBALS", "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetGet(ctx, nameStr)
	}
	return c.h.GetString(nameStr), nil
}

func (c *FuncContext) OffsetCheck(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, bool, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return nil, false, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		if c.this == nil {
			return nil, false, nil
		}
		return c.this.ZVal(), true, nil
	case "GLOBALS", "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetCheck(ctx, nameStr)
	}

	v, found := c.h.GetStringB(nameStr)
	if !found {
		return nil, false, nil
	}
	return v, true, nil
}

func (c *FuncContext) OffsetSet(ctx phpv.Context, name phpv.Val, v *phpv.ZVal) error {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		return ctx.Errorf("Cannot re-assign $this")
	}

	// Track object references: IncRef new object, DecRef old object.
	old := c.h.GetString(nameStr)
	isRef := old != nil && old.IsRef()

	var oldObj interface {
		DecRef(phpv.Context) error
	}
	if old != nil && old.GetType() == phpv.ZtObject {
		if obj, ok := old.Value().(interface {
			DecRef(phpv.Context) error
		}); ok {
			oldObj = obj
		}
	}
	// Only IncRef for non-reference direct object storage
	if !isRef && v != nil && v.GetType() == phpv.ZtObject && !v.IsRef() {
		if obj, ok := v.Value().(interface{ IncRef() }); ok {
			obj.IncRef()
		}
	}

	err := c.h.SetString(nameStr, v)
	if err != nil {
		return err
	}

	if oldObj != nil {
		return oldObj.DecRef(ctx)
	}
	return nil
}


func (c *FuncContext) OffsetUnset(ctx phpv.Context, name phpv.Val) error {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		return ctx.Errorf("Cannot unset $this")
	}
	return c.h.UnsetString(nameStr)
}

func (c *FuncContext) Count(ctx phpv.Context) phpv.ZInt {
	return c.h.Count()
}

func (c *FuncContext) NewIterator() phpv.ZIterator {
	return c.h.NewIterator()
}

func (ctx *FuncContext) Parent(n int) phpv.Context {
	if n <= 1 {
		return ctx.Context
	} else {
		return ctx.Context.Parent(n - 1)
	}
}

func (ctx *FuncContext) GetFuncName() string {
	name := ctx.c.Name()
	if ctx.class != nil && ctx.methodType != "" {
		// Native methods (e.g. built-in constructors) have an empty Name();
		// when called as a method they are constructors.
		if name == "" {
			name = "__construct"
		}
		// PHP uses :: in error messages/warning for both static and instance methods
		return string(ctx.class.GetName()) + "::" + name
	}
	return name
}

// GetFuncNameForTrace returns the function name using the actual method type (-> or ::) for stack traces
func (ctx *FuncContext) GetFuncNameForTrace() string {
	name := ctx.c.Name()
	if ctx.class != nil && ctx.methodType != "" {
		if name == "" {
			name = "__construct"
		}
		return string(ctx.class.GetName()) + ctx.methodType + name
	}
	return name
}

func (ctx *FuncContext) Error(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := ctx.Loc().Error(ctx, err, t...)
	result := phperr.HandleUserError(ctx, wrappedErr)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (ctx *FuncContext) Errorf(format string, a ...any) error {
	err := ctx.Loc().Errorf(ctx, phpv.E_ERROR, format, a...)
	result := phperr.HandleUserError(ctx, err)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (ctx *FuncContext) FuncError(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := ctx.Loc().Error(ctx, err, t...)
	wrappedErr.FuncName = ctx.GetFuncName()
	result := phperr.HandleUserError(ctx, wrappedErr)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}
func (ctx *FuncContext) FuncErrorf(format string, a ...any) error {
	err := ctx.Loc().Errorf(ctx, phpv.E_ERROR, format, a...)
	err.FuncName = ctx.GetFuncName()
	result := phperr.HandleUserError(ctx, err)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (ctx *FuncContext) Warn(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_WARNING))
	return logWarning(ctx, format, a...)
}

func (ctx *FuncContext) Notice(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	return logWarning(ctx, format, a...)
}

func (ctx *FuncContext) Deprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_DEPRECATED))
	err := logWarning(ctx, format, a...)
	if err == nil {
		ctx.Global().ShownDeprecated(format)
	}
	return err
}

func (ctx *FuncContext) UserDeprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_USER_DEPRECATED))
	return logWarning(ctx, format, a...)
}

func (ctx *FuncContext) WarnDeprecated() error {
	funcName := ctx.GetFuncName()
	if ok := ctx.Global().ShownDeprecated(funcName); ok {
		err := logWarning(
			ctx,
			"The %s() function is deprecated. This message will be suppressed on further calls",
			funcName, logopt.NoFuncName(true), logopt.ErrType(phpv.E_DEPRECATED),
		)
		return err
	}
	return nil
}

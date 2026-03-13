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

// Release returns the FuncContext to the pool after clearing it
func (c *FuncContext) Release() {
	c.Context = nil
	c.h.Empty()
	c.this = nil
	c.Args = c.Args[:0]
	c.c = nil
	c.loc = nil
	c.class = nil
	c.methodType = ""
	c.isInternal = false
	funcContextPool.Put(c)
}

type FuncContext struct {
	phpv.Context

	h    *phpv.ZHashTable
	this phpv.ZObject
	Args []*phpv.ZVal
	c    phpv.Callable // called object (this function itself)

	loc *phpv.Loc

	class      phpv.ZClass
	methodType string

	isInternal bool // true when called from internal code (e.g., output buffer callbacks)
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

	// Eagerly call __destruct when overwriting a variable that holds an object.
	if old := c.h.GetString(nameStr); old != nil && old.GetType() == phpv.ZtObject {
		if obj, ok := old.Value().(phpv.ZObject); ok {
			if _, hasDestructor := obj.GetClass().GetMethod("__destruct"); hasDestructor {
				err := c.h.SetString(nameStr, v)
				if err != nil {
					return err
				}
				if destructable, ok2 := obj.(interface {
					CallDestructor(phpv.Context) error
				}); ok2 {
					return destructable.CallDestructor(ctx)
				}
				return nil
			}
		}
	}

	return c.h.SetString(nameStr, v)
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
		// PHP uses :: in error messages/warning for both static and instance methods
		return string(ctx.class.GetName()) + "::" + name
	}
	return name
}

// GetFuncNameForTrace returns the function name using the actual method type (-> or ::) for stack traces
func (ctx *FuncContext) GetFuncNameForTrace() string {
	name := ctx.c.Name()
	if ctx.class != nil && ctx.methodType != "" {
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

package phpctx

import (
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

type FuncContext struct {
	phpv.Context

	h    *phpv.ZHashTable
	this phpv.ZObject
	Args []*phpv.ZVal
	c    phpv.Callable // called object (this function itself)

	loc *phpv.Loc

	class      phpv.ZClass
	methodType string
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

func (c *FuncContext) This() phpv.ZObject {
	return c.this
}

func (c *FuncContext) Class() phpv.ZClass {
	return c.class
}

func (c *FuncContext) OffsetExists(ctx phpv.Context, name phpv.Val) (bool, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return false, err
	}

	switch name.(phpv.ZString) {
	case "this":
		if c.this == nil {
			return false, nil
		}
		return true, nil
	case "GLOBALS":
		return true, nil
	case "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetExists(ctx, name)
	}
	return c.h.HasString(name.(phpv.ZString)), nil
}

func (c *FuncContext) OffsetGet(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	switch name.(phpv.ZString) {
	case "this":
		if c.this == nil {
			return nil, nil
		}
		return c.this.ZVal(), nil
	case "GLOBALS", "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetGet(ctx, name)
	}
	return c.h.GetString(name.(phpv.ZString)), nil
}

func (c *FuncContext) OffsetCheck(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, bool, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, false, err
	}

	switch name.(phpv.ZString) {
	case "this":
		if c.this == nil {
			return nil, false, nil
		}
		return c.this.ZVal(), true, nil
	case "GLOBALS", "_SERVER", "_GET", "_POST", "_FILES", "_COOKIE", "_SESSION", "_REQUEST", "_ENV":
		return c.Global().OffsetCheck(ctx, name)
	}

	if !c.h.HasString(name.(phpv.ZString)) {
		return nil, false, nil
	}
	return c.h.GetString(name.(phpv.ZString)), true, nil
}

func (c *FuncContext) OffsetSet(ctx phpv.Context, name phpv.Val, v *phpv.ZVal) error {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.(phpv.ZString) {
	case "this":
		return ctx.Errorf("Cannot re-assign $this")
	}
	return c.h.SetString(name.(phpv.ZString), v)
}

func (c *FuncContext) OffsetUnset(ctx phpv.Context, name phpv.Val) error {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.(phpv.ZString) {
	case "this":
		return ctx.Errorf("Cannot unset $this")
	}
	return c.h.UnsetString(name.(phpv.ZString))
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
	return ctx.c.Name()
}

func (ctx *FuncContext) Error(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := ctx.Loc().Error(ctx, err, t...)
	return phperr.HandleUserError(ctx, wrappedErr)
}

func (ctx *FuncContext) Errorf(format string, a ...any) error {
	err := ctx.Loc().Errorf(ctx, phpv.E_ERROR, format, a...)
	return phperr.HandleUserError(ctx, err)
}

func (ctx *FuncContext) FuncError(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := ctx.Loc().Error(ctx, err, t...)
	wrappedErr.FuncName = ctx.GetFuncName()
	return phperr.HandleUserError(ctx, wrappedErr)
}
func (ctx *FuncContext) FuncErrorf(format string, a ...any) error {
	err := ctx.Loc().Errorf(ctx, phpv.E_ERROR, format, a...)
	err.FuncName = ctx.GetFuncName()
	return phperr.HandleUserError(ctx, err)
}

func (ctx *FuncContext) Warn(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_WARNING))
	return logWarning(ctx, format, a...)
}

func (ctx *FuncContext) Notice(format string, a ...any) error {
	ctx.Global().WriteErr([]byte{'\n'})
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	return logWarning(ctx, format, a...)
}

func (ctx *FuncContext) Deprecated(format string, a ...any) error {
	ctx.Global().WriteErr([]byte{'\n'})
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
		ctx.Global().WriteErr([]byte{'\n'})
		err := logWarning(
			ctx,
			"The %s() function is deprecated. This message will be suppressed on further calls",
			funcName, logopt.NoFuncName(true), logopt.ErrType(phpv.E_DEPRECATED),
		)
		return err
	}
	return nil
}

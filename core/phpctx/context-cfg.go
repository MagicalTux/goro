package phpctx

import (
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

type cfgContext struct {
	phpv.Context

	k phpv.ZString
	v *phpv.ZVal
}

func WithConfig(parent phpv.Context, name phpv.ZString, v *phpv.ZVal) phpv.Context {
	return &cfgContext{parent, name, v}
}

func (c *cfgContext) GetConfig(name phpv.ZString, def *phpv.ZVal) *phpv.ZVal {
	if name == c.k {
		return c.v
	}
	return c.Context.GetConfig(name, def)
}

// Override Warn/Notice/Deprecated so logWarning receives this context
// (and thus sees the overridden GetConfig for error_reporting).
func (c *cfgContext) Warn(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_WARNING))
	return logWarning(c, format, a...)
}

func (c *cfgContext) Notice(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	return logWarning(c, format, a...)
}

func (c *cfgContext) Deprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_DEPRECATED))
	err := logWarning(c, format, a...)
	if err == nil {
		c.Global().ShownDeprecated(format)
	}
	return err
}

func (c *cfgContext) UserDeprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_USER_DEPRECATED))
	return logWarning(c, format, a...)
}

func (c *cfgContext) Parent(n int) phpv.Context {
	if n <= 1 {
		return c.Context
	}
	return c.Context.Parent(n - 1)
}

// funcNameContext wraps a context to override GetFuncName() without changing
// other context properties. This is used by call_user_func/call_user_func_array
// so that error messages reference the correct function name.
type funcNameContext struct {
	phpv.Context
	name string
}

// WithFuncName creates a context wrapper that overrides GetFuncName().
func WithFuncName(parent phpv.Context, name string) phpv.Context {
	return &funcNameContext{parent, name}
}

func (c *funcNameContext) GetFuncName() string {
	return c.name
}

func (c *funcNameContext) Parent(n int) phpv.Context {
	if n <= 1 {
		return c.Context
	}
	return c.Context.Parent(n - 1)
}

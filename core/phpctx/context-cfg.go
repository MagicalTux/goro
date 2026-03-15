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

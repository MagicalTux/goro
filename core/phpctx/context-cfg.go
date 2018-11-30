package phpctx

import "github.com/MagicalTux/goro/core/phpv"

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

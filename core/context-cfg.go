package core

type cfgContext struct {
	Context

	k ZString
	v *ZVal
}

func WithConfig(parent Context, name ZString, v *ZVal) Context {
	return &cfgContext{parent, name, v}
}

func (c *cfgContext) GetConfig(name ZString, def *ZVal) *ZVal {
	if name == c.k {
		return c.v
	}
	return c.Context.GetConfig(name, def)
}

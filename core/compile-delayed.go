package core

import "github.com/MagicalTux/goro/core/phpv"

type compileDelayed struct {
	v phpv.Runnable
}

func (c *compileDelayed) GetType() phpv.ZType {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) ZVal() *phpv.ZVal {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return c.v.Run(ctx)
}

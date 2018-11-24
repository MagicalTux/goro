package core

type compileDelayed struct {
	v Runnable
}

func (c *compileDelayed) GetType() ZType {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) ZVal() *ZVal {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) AsVal(ctx Context, t ZType) (Val, error) {
	panic("compileDelayed values should not be accessed")
}

func (c *compileDelayed) Run(ctx Context) (*ZVal, error) {
	return c.v.Run(ctx)
}

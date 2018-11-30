package phpv

type Compilable interface {
	Compile(ctx Context) error
}

type CompileDelayed struct {
	V Runnable
}

func (c *CompileDelayed) GetType() ZType {
	panic("CompileDelayed values should not be accessed")
}

func (c *CompileDelayed) ZVal() *ZVal {
	panic("CompileDelayed values should not be accessed")
}

func (c *CompileDelayed) AsVal(ctx Context, t ZType) (Val, error) {
	panic("CompileDelayed values should not be accessed")
}

func (c *CompileDelayed) String() string {
	panic("CompileDelayed values should not be accessed")
}

func (c *CompileDelayed) Run(ctx Context) (*ZVal, error) {
	return c.V.Run(ctx)
}

func (c *CompileDelayed) Value() Val {
	panic("CompileDelayed values should not be accessed")
}

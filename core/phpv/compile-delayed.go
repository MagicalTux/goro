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
	// No re-entry guard here. Circular reference detection is handled
	// by callers (e.g., cc.Resolving in runClassStaticObjRef.Run).
	// Removing the guard enables legitimate re-entrant resolution
	// when autoloading satisfies a dependency mid-resolution (GH-10709).
	return c.V.Run(ctx)
}

func (c *CompileDelayed) Value() Val {
	panic("CompileDelayed values should not be accessed")
}

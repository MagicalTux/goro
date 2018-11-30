package compiler

type zclosureCompileCtx struct {
	compileCtx
	closure *ZClosure
}

func (z *zclosureCompileCtx) getFunc() *ZClosure {
	return z.closure
}

package compiler

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type zclosureCompileCtx struct {
	compileCtx
	closure *ZClosure
}

func (z *zclosureCompileCtx) getFunc() *ZClosure {
	return z.closure
}

func (z *zclosureCompileCtx) getClass() *phpobj.ZClass {
	return z.compileCtx.getClass()
}

func (z *zclosureCompileCtx) getNamespace() phpv.ZString {
	return z.compileCtx.getNamespace()
}

func (z *zclosureCompileCtx) resolveClassName(name phpv.ZString) phpv.ZString {
	return z.compileCtx.resolveClassName(name)
}

func (z *zclosureCompileCtx) resolveFunctionName(name phpv.ZString) phpv.ZString {
	return z.compileCtx.resolveFunctionName(name)
}

func (z *zclosureCompileCtx) resolveConstantName(name string) string {
	return z.compileCtx.resolveConstantName(name)
}

func (z *zclosureCompileCtx) isTopLevel() bool {
	return false // inside a function is never top level
}

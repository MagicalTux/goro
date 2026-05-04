package vm

import (
	"github.com/KarpelesLab/goro/core/phpv"
)

// callUser dispatches an OP_CALL_USER. The function name is looked up
// via Global.GetFunction at call time (not compile time), matching the
// AST behaviour for runnableFunctionCall.Run. Args are popped from the
// VM stack as a freshly-allocated []*phpv.ZVal slice.
//
// By-ref parameters: Goro's CallZVal does the binding internally based
// on the callable's FuncArg metadata. Passing pre-evaluated ZVals here
// means by-ref parameters won't propagate mutations back to caller
// locals — for the MVP scope this is acceptable (none of the
// supported call sites use by-ref). The bytecode emitter falls back to
// AST when it detects a callable's signature requires by-ref.
func (f *Frame) callUser(ctx phpv.Context, name phpv.ZString, argc int) error {
	args := make([]*phpv.ZVal, argc)
	for i := argc - 1; i >= 0; i-- {
		args[i] = f.pop()
	}
	callable, err := ctx.Global().GetFunction(ctx, name)
	if err != nil {
		return err
	}
	res, err := ctx.CallZVal(ctx, callable, args, nil)
	if err != nil {
		return err
	}
	if res == nil {
		res = phpv.ZNULL.ZVal()
	}
	f.push(res)
	return nil
}

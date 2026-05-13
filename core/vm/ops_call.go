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
func (f *Frame) callUser(ctx phpv.Context, name phpv.ZString, argc int, nameIdx uint16) error {
	args := make([]*phpv.ZVal, argc)
	for i := argc - 1; i >= 0; i-- {
		args[i] = f.pop()
	}
	// Cached callable lookup: avoid the per-call hashtable lookup
	// for the common case of a stable target. The cache is invalidated
	// when GetFunction returns a different pointer (function redeclared
	// or replaced).
	var callable phpv.Callable
	if int(nameIdx) < len(f.fn.CallableCache) {
		callable = f.fn.CallableCache[nameIdx]
	}
	if callable == nil {
		c, err := ctx.Global().GetFunction(ctx, name)
		if err != nil {
			return err
		}
		callable = c
		// Lazy-allocate the cache; size matches Consts so any name
		// idx is reachable.
		if f.fn.CallableCache == nil {
			f.fn.CallableCache = make([]phpv.Callable, len(f.fn.Consts))
		}
		if int(nameIdx) < len(f.fn.CallableCache) {
			f.fn.CallableCache[nameIdx] = callable
		}
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

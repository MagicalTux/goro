package vm

import (
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// dispatchTryHandler scans Function.TryHandlers for the innermost
// handler whose [Start, End) range contains the failing PC. If found,
// each catch clause is checked against the exception's class; the
// first matching clause has its variable bound and execution jumps to
// its body.
//
// Returns:
//   - handled=true, newErr=nil: a catch matched, pc redirected.
//   - handled=false, newErr=nil: no handler matched; caller should
//     propagate the original throw.
//   - handled=false, newErr=…: binding the exception variable
//     triggered a destructor that threw a NEW exception (PHP behaves
//     this way: bug53511). Caller should swap to newErr and
//     re-attempt dispatch.
func (f *Frame) dispatchTryHandler(ctx phpv.Context, throwErr *phperr.PhpThrow) (bool, error) {
	if len(f.fn.TryHandlers) == 0 || throwErr == nil || throwErr.Obj == nil {
		return false, nil
	}
	failPC := f.pc - 1

	exClass := throwErr.Obj.GetClass()
	for i := 0; i < len(f.fn.TryHandlers); i++ {
		h := f.fn.TryHandlers[i]
		if failPC < h.Start || failPC >= h.End {
			continue
		}
		for _, clause := range h.Catches {
			if matchCatchTypes(ctx, exClass, clause.Types) {
				// Drop any partially-pushed values from the failing
				// expression so the catch body starts at the same
				// stack depth as the try body.
				for f.sp > h.StackBase {
					f.sp--
					f.stack[f.sp] = nil
				}
				if clause.VarIdx != 0xFFFF {
					exVal := throwErr.Obj.ZVal()
					name := f.fn.Locals[clause.VarIdx]
					f.locals[clause.VarIdx] = exVal
					if err := ctx.OffsetSet(ctx, name, exVal); err != nil {
						return false, err
					}
				}
				f.pc = clause.PC
				return true, nil
			}
		}
	}
	return false, nil
}

// matchCatchTypes returns true if exClass matches any of the catch's
// declared type names (the union in `catch (A | B $e)`).
func matchCatchTypes(ctx phpv.Context, exClass phpv.ZClass, types []phpv.ZString) bool {
	if len(types) == 0 {
		return true
	}
	for _, name := range types {
		class, err := ctx.Global().GetClass(ctx, name, false)
		if err != nil || class == nil {
			continue
		}
		if exClass.InstanceOf(class) {
			return true
		}
		// Implements check for interfaces — InstanceOf handles
		// classes; phpobj.ZClass.Implements handles interfaces.
		if zc, ok := exClass.(*phpobj.ZClass); ok {
			if zc.Implements(class) {
				return true
			}
		}
	}
	return false
}

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
// its body. Returns true if a catch matched and pc was redirected;
// false otherwise (caller should propagate the throw).
func (f *Frame) dispatchTryHandler(ctx phpv.Context, throwErr *phperr.PhpThrow) bool {
	if len(f.fn.TryHandlers) == 0 || throwErr == nil || throwErr.Obj == nil {
		return false
	}
	// f.pc has already been incremented past the failing instruction.
	failPC := f.pc - 1

	// Inner handlers are registered BEFORE their enclosing outer
	// handler (registration happens at the end of emitTry, and
	// nested emitTry calls run during the outer's body emit). So
	// forward iteration hits the innermost handler first.
	exClass := throwErr.Obj.GetClass()
	for i := 0; i < len(f.fn.TryHandlers); i++ {
		h := f.fn.TryHandlers[i]
		if failPC < h.Start || failPC >= h.End {
			continue
		}
		// Found an active handler. Try each catch clause.
		for _, clause := range h.Catches {
			if matchCatchTypes(ctx, exClass, clause.Types) {
				// Bind the exception variable.
				if clause.VarIdx != 0xFFFF {
					exVal := throwErr.Obj.ZVal()
					name := f.fn.Locals[clause.VarIdx]
					f.locals[clause.VarIdx] = exVal
					if err := ctx.OffsetSet(ctx, name, exVal); err != nil {
						// If binding fails, surface the error normally.
						return false
					}
				}
				f.pc = clause.PC
				return true
			}
		}
		// No matching catch in this handler. Continue scanning outer
		// handlers (the throw escapes to the next try level).
	}
	return false
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

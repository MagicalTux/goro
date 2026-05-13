package vmcompiler

import (
	"errors"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// Enabled reports whether the VM is enabled. The VM is now the only
// runtime; this always returns true. Kept for backwards-compatible
// callers in tests / external code.
func Enabled() bool { return true }

// SetEnabled is a no-op kept for backwards-compatible callers. The VM
// can't be disabled — the AST tree-walker is no longer reachable as a
// standalone runtime. (AST node Run() methods still exist as
// infrastructure that the VM delegates to for nodes it doesn't lower
// piecewise.)
func SetEnabled(bool) {}

func init() {
	compiler.TryBuildVMClosureBody = tryBuildClosureBody
	compiler.TryBuildVMScript = tryBuildScript

	// Lazy-aware by-ref probe: lets the VM emitter check whether a
	// user function (including not-yet-registered lazy entries) takes
	// any by-reference parameters, without triggering the lazy
	// registration side effect.
	compiler.LookupLazyByRef = func(g phpv.GlobalContext, name phpv.ZString) (bool, bool) {
		if gg, ok := g.(*phpctx.Global); ok {
			return gg.LookupByRef(name)
		}
		return false, false
	}
}

// tryBuildClosureBody bytecode-compiles a closure body. Returns nil
// only when the emitter can't lower the body (ErrUnsupported); in
// that case the compiler keeps the AST body — note this is an
// internal fall-back for genuinely unsupported constructs, not a
// user-facing runtime selection.
func tryBuildClosureBody(ctx phpv.Context, name phpv.ZString, src *phpv.Loc, body phpv.Runnable, hasByRef bool) phpv.Runnable {
	fn, err := Compile(name, src, body, ctx)
	if err != nil {
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		// Any other error: don't crash the compile, just decline.
		return nil
	}
	if hasByRef {
		// By-ref params/captures need OffsetSet to propagate through
		// the ref wrapper; skipping the hashtable mirror would
		// silently drop the propagation.
		fn.SlotOnly = false
	}
	return vm.WrapClosureBody(fn, body)
}


// tryBuildScript wraps a top-level script's Runnables in a VM runner.
// Returns nil only for genuinely unsupported constructs (same
// fall-back semantics as tryBuildClosureBody).
func tryBuildScript(ctx phpv.Context, src *phpv.Loc, body phpv.Runnable) phpv.Runnable {
	fn, err := Compile("<main>", src, body, ctx)
	if err != nil {
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		return nil
	}
	return vm.Wrap(fn, body)
}

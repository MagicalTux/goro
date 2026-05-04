package vm

import (
	"io"

	"github.com/KarpelesLab/goro/core/phpv"
)

// Runnable wraps a *Function so it satisfies the phpv.Runnable contract
// the rest of the engine expects. Top-level scripts and VM-compiled
// closure bodies plug in through this wrapper without touching the
// AST execution pipeline.
type Runnable struct {
	Fn *Function
	// Fallback, when non-nil, is the original AST tree the bytecode
	// was generated from. It's used by Dump (for accurate source
	// rendering) and is the safety net we'd switch to if a runtime
	// invariant ever forced us to bail out of the VM.
	Fallback phpv.Runnable
}

// Wrap returns a Runnable that executes fn through the VM and falls
// back to fallback for non-execution surface area like Dump.
func Wrap(fn *Function, fallback phpv.Runnable) *Runnable {
	return &Runnable{Fn: fn, Fallback: fallback}
}

// Inner returns the AST tree the wrapper was built from. Lets
// compiler-internal walks (Dump, GetChildren, validate-break, …)
// continue to operate on the original structure.
func (r *Runnable) Inner() phpv.Runnable { return r.Fallback }

// Run executes the wrapped function via the VM dispatcher.
func (r *Runnable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return Run(ctx, r.Fn)
}

// Dump delegates to the AST fallback when available so debug output
// matches the user's original source. If no fallback was provided
// (hand-built Function for tests, e.g.), Dump emits the disassembly.
func (r *Runnable) Dump(w io.Writer) error {
	if r.Fallback != nil {
		return r.Fallback.Dump(w)
	}
	return DumpFunction(w, r.Fn)
}

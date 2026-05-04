package vm

import (
	"io"

	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
)

// ClosureBody wraps a VM Function for use as the body of a *ZClosure.
// It implements phpv.Runnable, but its Run never returns (value, nil)
// the way Function execution normally would: instead it converts an
// OpRet result into a *phperr.PhpReturn error so the existing
// ZClosure.callBody path (which only inspects the error) handles it
// uniformly with AST function bodies.
//
// Use Wrap when you want a Runnable that returns its value through the
// normal (value, nil) channel — e.g. for top-level scripts.
type ClosureBody struct {
	Fn *Function
	// AST is the original AST tree the bytecode was generated from.
	// It is kept so that compiler-internal walks (Dump, static-var
	// collection, attribute introspection, …) can still inspect the
	// source structure. Execution always goes through Fn.
	AST phpv.Runnable
}

// WrapClosureBody returns a Runnable suitable for assignment to
// ZClosure.code. ast is the original AST tree before compilation; the
// wrapper keeps it for non-execution introspection paths.
func WrapClosureBody(fn *Function, ast phpv.Runnable) phpv.Runnable {
	return &ClosureBody{Fn: fn, AST: ast}
}

// Inner returns the AST tree the body was compiled from. This lets
// compiler-internal walks (collectStaticVars, validateBreakContinue,
// Dump pretty-printing) keep operating on the original structure.
func (c *ClosureBody) Inner() phpv.Runnable { return c.AST }

// Run executes the function and converts the result into a PhpReturn
// error so callBody picks it up via phperr.CatchReturn.
//
// If the VM returned (nil, err) where err is already a PhpReturn or
// any non-internal error, propagate as-is.
func (c *ClosureBody) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	res, err := Run(ctx, c.Fn)
	if err != nil {
		return nil, err
	}
	if res == nil {
		res = phpv.ZNULL.ZVal()
	}
	return nil, &phperr.PhpReturn{V: res}
}

// Dump prefers the AST when available so debug output matches user
// source; falls back to bytecode disassembly otherwise.
func (c *ClosureBody) Dump(w io.Writer) error {
	if c.AST != nil {
		return c.AST.Dump(w)
	}
	return DumpFunction(w, c.Fn)
}

package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// Compile walks an AST tree (the result of compiler.Compile or the body
// of a *ZClosure) and produces a *vm.Function. Returns ErrUnsupported
// (wrapped) if it hits a node it doesn't yet understand.
//
// name is used for stack traces / disassembly. source is the location
// of the unit being compiled (e.g. the start of the script or the
// `function ...(...)` keyword).
func Compile(name phpv.ZString, source *phpv.Loc, body phpv.Runnable) (*vm.Function, error) {
	// Unwrap any pre-existing VM wrapper. This happens when tests or
	// callers pass a Runnable that already went through a script-level
	// VM wrap from compiler.Compile (when GORO_VM is on).
	if inner, ok := body.(interface{ Inner() phpv.Runnable }); ok {
		body = inner.Inner()
	}

	e := newEmitter(source)

	// Treat the body as a list of statements. If it's already a
	// Runnables we walk it directly; otherwise wrap it.
	if rs, ok := body.(phpv.Runnables); ok {
		if err := e.emitStmts(rs); err != nil {
			return nil, err
		}
	} else if body != nil {
		if err := e.emitStmt(body); err != nil {
			return nil, err
		}
	}

	// Always end with RET_NULL so falling off the end produces null
	// (matches PHP's "function with no explicit return" behaviour).
	e.emit(vm.OpRetNull, 0, 0, 0)

	return e.finish(name, 0), nil
}

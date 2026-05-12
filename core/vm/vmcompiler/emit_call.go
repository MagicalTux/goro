package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// funcCallNode matches *compiler.runnableFunctionCall.
type funcCallNode interface {
	FuncCallName() phpv.ZString
	FuncCallArgs() []phpv.Runnable
	FuncCallLoc() *phpv.Loc
}

// funcCallRefNode matches *compiler.runnableFunctionCallRef — calls
// where the callable comes from an expression ($f() with $f a
// variable, or $obj->method() chained from a result).
type funcCallRefNode interface {
	FuncCallRefExpr() phpv.Runnable
	FuncCallRefArgs() []phpv.Runnable
	FuncCallRefLoc() *phpv.Loc
}

func (e *emitter) emitFunctionCall(n funcCallNode) error {
	name := n.FuncCallName()
	if name == "" {
		// Dynamic call ($f()) — handled by runnableFunctionCallRef in the
		// AST. Falls back for now.
		return unsupportedf("dynamic function call (variable name)")
	}
	// Builtins that take by-ref args can't be called via the VM's
	// value-passing protocol (CallZVal). Fall back to AST so they
	// still mutate the caller's variables.
	if compiler.ByRefBuiltins[name.ToLower()] {
		return unsupportedf("call to by-ref builtin %s", name)
	}
	// User-defined functions with by-ref params: same problem.
	// We can query the global function table when the emitter has
	// a Context available (set when the closure-body hook is
	// invoked with the compile context).
	if e.ctx != nil {
		if g := e.ctx.Global(); g != nil && compiler.FunctionTakesByRef(g, name) {
			return unsupportedf("call to by-ref user function %s", name)
		}
	}

	args := n.FuncCallArgs()
	if len(args) > 0xFFFF {
		return unsupportedf("function call with too many args (>=65536)")
	}

	// Reject named-argument and spread-argument wrappers — those are
	// distinct AST node types implementing phpv.NamedArgument /
	// SpreadArgument. Detect via interface assertion.
	for _, a := range args {
		if _, ok := a.(phpv.NamedArgument); ok {
			return unsupportedf("named argument in call")
		}
		if _, ok := a.(phpv.SpreadArgument); ok {
			return unsupportedf("spread argument in call")
		}
	}

	// Evaluate args left-to-right and push them onto the VM stack.
	for _, a := range args {
		if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
			return err
		}
	}

	// Look up the function name in the const pool.
	nameIdx := e.constIndex(name)

	// Emit CALL_USER: A=name idx, B=argc.
	e.emit(vm.OpCallUser, nameIdx, uint16(len(args)), 0)
	// argc args popped, 1 result pushed → net delta -(argc-1).
	if len(args) > 0 {
		e.popStack(len(args) - 1)
	} else {
		e.pushStack(1)
	}

	// In statement context the result isn't used; drop it.
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

// emitFunctionCallRef lowers `$expr(args…)` calls. The expression is
// evaluated and the resulting value is resolved as a callable at
// runtime via compiler.ResolveCallable.
func (e *emitter) emitFunctionCallRef(n funcCallRefNode) error {
	args := n.FuncCallRefArgs()
	if len(args) > 0xFFFF {
		return unsupportedf("indirect call with too many args")
	}
	for _, a := range args {
		if _, ok := a.(phpv.NamedArgument); ok {
			return unsupportedf("named argument in indirect call")
		}
		if _, ok := a.(phpv.SpreadArgument); ok {
			return unsupportedf("spread argument in indirect call")
		}
	}

	// Push callable, then args.
	if err := e.withSubexpr(func() error { return e.emitExpr(n.FuncCallRefExpr()) }); err != nil {
		return err
	}
	for _, a := range args {
		if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
			return err
		}
	}
	e.emit(vm.OpCallIndirect, 0, uint16(len(args)), 0)
	// Pops 1 callable + argc args; pushes 1 result. Net delta: -argc.
	if len(args) > 0 {
		e.popStack(len(args))
	}

	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

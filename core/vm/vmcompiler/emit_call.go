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

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
	args := n.FuncCallArgs()

	// AST-delegate calls whose shape we don't lower piecewise:
	// named/spread args, or a callee that takes by-ref params.
	// Note: unknown-callee-with-writable-arg was tried as a
	// conservative gate, but the IsSlotSafe override it required
	// broke too many independent tests; keep the narrower check.
	byRefBuiltin := compiler.ByRefBuiltins[name.ToLower()]
	byRefUser := false
	if e.ctx != nil {
		if g := e.ctx.Global(); g != nil {
			byRefUser = compiler.FunctionTakesByRef(g, name)
		}
	}
	if byRefBuiltin || byRefUser || compiler.CallHasSpecialArgs(args) {
		// Route the call through a dedicated by-exprs opcode so
		// ctx.Call sees the raw argument expressions and applies its
		// by-ref / named / spread binding pipeline. Net effect: no
		// generic AST.Run() delegation, but the binding behaviour
		// matches the AST path byte-for-byte.
		return e.emitCallByExprs(name, args)
	}

	if len(args) > 0xFFFF {
		return unsupportedf("function call with too many args (>=65536)")
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
	// Indirect calls go through ResolveCallable at runtime, so we
	// don't know whether the callable expects by-ref args. AST-
	// delegate when any arg is a writable lvalue so a possible
	// by-ref param binds correctly.
	if compiler.CallHasSpecialArgs(args) || compiler.CallHasWritableArg(args) {
		return e.emitCallIndirectByExprs(n.FuncCallRefExpr(), args)
	}
	if len(args) > 0xFFFF {
		return unsupportedf("indirect call with too many args")
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

// emitCallByExprs lowers a named function call whose argument shape
// requires the full ctx.Call binding pipeline (by-ref params, named
// arguments, spread). Args are NOT evaluated by the emitter — they're
// stashed in fn.SubArgs and handed to ctx.Call at runtime.
//
// Stmt-context: drop the result with OP_POP after the call. This
// matches the AST runner which always pushes a return value and
// expression-statement evaluation discards it.
func (e *emitter) emitCallByExprs(name phpv.ZString, args []phpv.Runnable) error {
	if len(args) > 0xFFFF {
		return unsupportedf("by-exprs call with too many args (>=65536)")
	}
	nameIdx := e.constIndex(name)
	argsIdx := e.subArgsIndex(args)
	e.emit(vm.OpCallUserByExprs, nameIdx, argsIdx, 0)
	// Pushes 1 result; no other stack effect (args weren't evaluated
	// to the stack — they live in SubArgs).
	e.pushStack(1)
	// Call may invalidate slot cache (e.g. by-ref param mutated caller
	// local), so refresh.
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

// emitCallIndirectByExprs is the by-exprs variant of OpCallIndirect.
// The callable expression is evaluated to a value on the stack; the
// runtime pops it, resolves it via ResolveCallable, then dispatches
// via ctx.Call with the AST argument expressions so by-ref / named /
// spread args bind correctly.
func (e *emitter) emitCallIndirectByExprs(callableExpr phpv.Runnable, args []phpv.Runnable) error {
	if len(args) > 0xFFFF {
		return unsupportedf("by-exprs indirect call with too many args (>=65536)")
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(callableExpr) }); err != nil {
		return err
	}
	argsIdx := e.subArgsIndex(args)
	e.emit(vm.OpCallIndirectByExprs, argsIdx, 0, 0)
	// Pops 1 callable, pushes 1 result. Net stack delta: 0.
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

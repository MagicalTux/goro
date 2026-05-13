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

// emitCallViaAST AST-delegates a function/method call when its shape
// (named/spread args, by-ref params, dynamic name, nullsafe) isn't
// lowered piecewise. The whole AST node runs as one opaque step
// pushing the result. Body must be flagged slot-unsafe by IsSlotSafe.
func (e *emitter) emitCallViaAST(raw phpv.Runnable) error {
	stmtCtx := e.stmtCtx
	idx := e.astIndex(raw)
	if stmtCtx {
		e.emit(vm.OpTryFinally, idx, 0, 0)
	} else {
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
	}
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	return nil
}

// emitCallViaASTWithSyncedLocals AST-delegates a call after writing
// each variable-arg local to the FuncContext hashtable. Used when the
// callable's by-ref signature isn't known at compile time and the
// surrounding body is slot-only — the AST runner needs to read
// up-to-date local values for those specific args to handle by-ref
// binding correctly. Targeted form: only the named locals are synced
// (not the bulk OP_SYNC_SLOTS approach which had broader side effects).
func (e *emitter) emitCallViaASTWithSyncedLocals(raw phpv.Runnable, args []phpv.Runnable) error {
	for _, a := range args {
		if v, ok := a.(variableNode); ok {
			idx := e.localIndex(v.VariableName())
			e.emit(vm.OpSyncLocal, idx, 0, 0)
		}
	}
	return e.emitCallViaAST(raw)
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
	byRefBuiltin := compiler.ByRefBuiltins[name.ToLower()]
	byRefUser := false
	unknownCallable := true
	if e.ctx != nil {
		if g := e.ctx.Global(); g != nil {
			if hit, br := compiler.LookupLazyByRefSafe(g, name); hit {
				unknownCallable = false
				byRefUser = br
			}
		}
	}
	if byRefBuiltin || byRefUser || compiler.CallHasSpecialArgs(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaAST(raw)
	}
	// Unknown-callable + variable-arg: emit OP_SYNC_LOCAL for each
	// variable arg so the AST runner can read its value from the
	// hashtable (the slot-only optimization normally skips the
	// mirror). Then AST-delegate. The targeted sync avoids the
	// FuncContext side effects of a bulk OP_SYNC_SLOTS.
	if unknownCallable && compiler.CallHasWritableArg(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaASTWithSyncedLocals(raw, args)
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
	if compiler.CallHasSpecialArgs(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("indirect call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaAST(raw)
	}
	// Indirect calls go through a runtime-resolved callable, so the
	// by-ref signature is unknowable. Sync any variable-arg locals
	// and AST-delegate when writable args are present.
	if compiler.CallHasWritableArg(args) {
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("indirect call AST delegation: cannot retrieve raw Runnable")
		}
		return e.emitCallViaASTWithSyncedLocals(raw, args)
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

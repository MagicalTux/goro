package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// --- statement-level accessor interfaces ------------------------------

type ifNode interface {
	IfCond() phpv.Runnable
	IfYes() phpv.Runnable
	IfNo() phpv.Runnable
	IfLoc() *phpv.Loc
	IfIsTernary() bool
	IfIsShortTernary() bool
}

type forNode interface {
	ForStart() phpv.Runnables
	ForCond() phpv.Runnables
	ForEach() phpv.Runnables
	ForCode() phpv.Runnable
	ForLoc() *phpv.Loc
}

type whileNode interface {
	WhileCond() phpv.Runnable
	WhileCode() phpv.Runnable
	WhileLoc() *phpv.Loc
}

type foreachNode interface {
	ForeachSrc() phpv.Runnable
	ForeachKey() phpv.Runnable
	ForeachValue() phpv.Runnable
	ForeachCode() phpv.Runnable
	ForeachIsRef() bool
	ForeachLoc() *phpv.Loc
}

type returnNode interface {
	ReturnValue() phpv.Runnable
	ReturnLoc() *phpv.Loc
	ReturnHasTypeHint() bool
}

type throwNode interface {
	ThrowValue() phpv.Runnable
	ThrowLoc() *phpv.Loc
}

// emitStmt emits bytecode for a statement-level node. Statements never
// leave a value on the stack on completion.
func (e *emitter) emitStmt(node phpv.Runnable) error {
	if node == nil {
		return nil
	}

	// Unwrap NoDiscard wrapper. Means the NoDiscard warning won't
	// fire from VM-compiled call sites — acceptable for now.
	if u, ok := node.(interface {
		NoDiscardInner() phpv.Runnable
	}); ok {
		node = u.NoDiscardInner()
	}

	// Runnables (slice of statements): emit each.
	if rs, ok := node.(phpv.Runnables); ok {
		return e.emitStmts(rs)
	}

	// Tick before each statement (matches Runnables.Run). Always emit
	// OP_TICK so the runtime updates ctx.Loc() (which exception
	// constructors and backtrace builders read) — when the node
	// doesn't expose a Loc(), the dispatcher falls back to fn.Source.
	if loc := stmtLoc(node); loc != nil {
		e.recordLoc(loc)
	}
	e.emit(vm.OpTick, 0, 0, 0)

	// Statement-specific dispatch.
	switch n := node.(type) {
	case ifNode:
		return e.emitIf(n)
	case whileNode:
		return e.emitWhile(n)
	case forNode:
		return e.emitFor(n)
	case foreachNode:
		return e.emitForeach(n)
	case returnNode:
		return e.emitReturn(n)
	case throwNode:
		return e.emitThrow(n)
	case compiler.TryNode:
		// try with finally is delegated wholesale to the AST runner —
		// the AST handles the intricate dance of running finally on
		// every exit path (normal completion, caught exception,
		// uncaught exception, return/break/continue out of the try).
		// Replicating that bytecode-natively would be a large amount
		// of code for a niche feature.
		if n.TryFinally() != nil {
			idx := e.astIndex(node)
			e.emit(vm.OpTryFinally, idx, 0, 0)
			return nil
		}
		return e.emitTry(n)
	case *phperr.PhpBreak:
		return e.emitBreak(n)
	case *phperr.PhpContinue:
		return e.emitContinue(n)
	}

	// Anything else: treat as expression statement (result discarded).
	prev := e.stmtCtx
	e.stmtCtx = true
	defer func() { e.stmtCtx = prev }()

	prevDepth := e.curStack
	if err := e.emitExpr(node); err != nil {
		return err
	}
	// The expression may have left a value on the stack (depending on
	// the node type — emitOperator in stmt mode for compound-assign
	// already pops, but a bare expression like `foo();` leaves one).
	// Drop any residual.
	if e.curStack > prevDepth {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

func (e *emitter) emitStmts(rs phpv.Runnables) error {
	for _, s := range rs {
		if err := e.emitStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *emitter) emitIf(n ifNode) error {
	// Plain `if` (and ternary expressions when used as statements).
	// Ternary as a value-producing expression isn't emitted here.

	// Emit cond.
	if err := e.withSubexpr(func() error { return e.emitExpr(n.IfCond()) }); err != nil {
		return err
	}

	// JMP_IF_FALSE → else label
	jmpFalse := e.emit(vm.OpJmpIfFalse, 0, 0, 0)
	e.popStack(1)

	// then-branch
	if err := e.emitStmt(n.IfYes()); err != nil {
		return err
	}

	if n.IfNo() == nil {
		// no else → patch jmpFalse to here
		e.patchJump(jmpFalse, uint32(len(e.code)))
		return nil
	}

	// JMP → end
	jmpEnd := e.emit(vm.OpJmp, 0, 0, 0)
	// patch jmpFalse → start of else
	e.patchJump(jmpFalse, uint32(len(e.code)))
	// else-branch
	if err := e.emitStmt(n.IfNo()); err != nil {
		return err
	}
	// patch jmpEnd → here
	e.patchJump(jmpEnd, uint32(len(e.code)))
	return nil
}

func (e *emitter) emitWhile(n whileNode) error {
	loop := e.pushLoop()
	loopHead := uint32(len(e.code))

	// cond
	if err := e.withSubexpr(func() error { return e.emitExpr(n.WhileCond()) }); err != nil {
		return err
	}
	exitJmp := e.emit(vm.OpJmpIfFalse, 0, 0, 0)
	e.popStack(1)

	// body
	if err := e.emitStmt(n.WhileCode()); err != nil {
		return err
	}

	// continue target = loopHead (re-evaluate cond).
	for _, pc := range loop.continuePCs {
		e.patchJump(pc, loopHead)
	}

	// jump back to head
	jmpBack := e.emit(vm.OpJmp, 0, 0, 0)
	e.patchJump(jmpBack, loopHead)

	// exit
	exit := uint32(len(e.code))
	e.patchJump(exitJmp, exit)
	for _, pc := range loop.breakPCs {
		e.patchJump(pc, exit)
	}
	e.popLoop()
	return nil
}

func (e *emitter) emitFor(n forNode) error {
	// init exprs (run once, results discarded)
	prev := e.stmtCtx
	e.stmtCtx = true
	for _, s := range n.ForStart() {
		prevDepth := e.curStack
		if err := e.emitExpr(s); err != nil {
			return err
		}
		if e.curStack > prevDepth {
			e.emit(vm.OpPop, 0, 0, 0)
			e.popStack(1)
		}
	}
	e.stmtCtx = prev

	loop := e.pushLoop()
	loopHead := uint32(len(e.code))

	// cond — for `for(;;)` cond may be empty: skip cond test entirely.
	exitJmps := []uint32{}
	conds := n.ForCond()
	for i, c := range conds {
		if err := e.withSubexpr(func() error { return e.emitExpr(c) }); err != nil {
			return err
		}
		if i == len(conds)-1 {
			// last cond's result decides whether to exit.
			exitJmps = append(exitJmps, e.emit(vm.OpJmpIfFalse, 0, 0, 0))
			e.popStack(1)
		} else {
			// Comma-separated: results discarded except the last.
			e.emit(vm.OpPop, 0, 0, 0)
			e.popStack(1)
		}
	}

	// body
	if err := e.emitStmt(n.ForCode()); err != nil {
		return err
	}

	// continue target = step exprs.
	stepStart := uint32(len(e.code))
	for _, pc := range loop.continuePCs {
		e.patchJump(pc, stepStart)
	}

	// step exprs (results discarded)
	prev = e.stmtCtx
	e.stmtCtx = true
	for _, s := range n.ForEach() {
		prevDepth := e.curStack
		if err := e.emitExpr(s); err != nil {
			return err
		}
		if e.curStack > prevDepth {
			e.emit(vm.OpPop, 0, 0, 0)
			e.popStack(1)
		}
	}
	e.stmtCtx = prev

	// jump back to head
	jmpBack := e.emit(vm.OpJmp, 0, 0, 0)
	e.patchJump(jmpBack, loopHead)

	// exit
	exit := uint32(len(e.code))
	for _, pc := range exitJmps {
		e.patchJump(pc, exit)
	}
	for _, pc := range loop.breakPCs {
		e.patchJump(pc, exit)
	}
	e.popLoop()
	return nil
}

func (e *emitter) emitForeach(n foreachNode) error {
	if n.ForeachIsRef() {
		return unsupportedf("foreach by reference (&$v)")
	}
	// Loop variable must be a simple local. Anything else (array
	// element, object property, list destructure) falls back.
	valNode, ok := n.ForeachValue().(variableNode)
	if !ok {
		return unsupportedf("foreach value target %T (only $local supported)", n.ForeachValue())
	}
	valIdx := e.localIndex(valNode.VariableName())

	keyIdx := uint16(0xFFFF)
	if k := n.ForeachKey(); k != nil {
		kn, ok := k.(variableNode)
		if !ok {
			return unsupportedf("foreach key target %T (only $local supported)", k)
		}
		keyIdx = e.localIndex(kn.VariableName())
	}

	// Eval src and emit the init op. C is patched to point past the
	// loop in case src isn't iterable (warning + skip).
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ForeachSrc()) }); err != nil {
		return err
	}
	initPC := e.emit(vm.OpForeachInit, 0, 0, 0)
	e.popStack(1) // pops src

	loop := e.pushLoop()
	loopHead := uint32(len(e.code))

	// Step: jumps to unwind on iterator exhaustion.
	stepPC := e.emit(vm.OpForeachStep, valIdx, keyIdx, 0)

	// Body
	if err := e.emitStmt(n.ForeachCode()); err != nil {
		return err
	}

	// continue → advance + jump to step
	contStart := uint32(len(e.code))
	for _, pc := range loop.continuePCs {
		e.patchJump(pc, contStart)
	}
	e.emit(vm.OpForeachAdvance, 0, 0, 0)
	jmpBack := e.emit(vm.OpJmp, 0, 0, 0)
	e.patchJump(jmpBack, loopHead)

	// Unwind target — break and natural-end land here.
	unwindStart := uint32(len(e.code))
	e.patchJump(stepPC, unwindStart)
	for _, pc := range loop.breakPCs {
		e.patchJump(pc, unwindStart)
	}
	e.emit(vm.OpForeachUnwind, 0, 0, 0)

	// End — init's "not iterable" jump lands here.
	end := uint32(len(e.code))
	e.patchJump(initPC, end)

	e.popLoop()
	return nil
}

func (e *emitter) emitReturn(n returnNode) error {
	if n.ReturnHasTypeHint() {
		return unsupportedf("return with type hint requires AST coercion")
	}
	if v := n.ReturnValue(); v != nil {
		if err := e.withSubexpr(func() error { return e.emitExpr(v) }); err != nil {
			return err
		}
		e.emit(vm.OpRet, 0, 0, 0)
		e.popStack(1)
	} else {
		e.emit(vm.OpRetNull, 0, 0, 0)
	}
	return nil
}

func (e *emitter) emitThrow(n throwNode) error {
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ThrowValue()) }); err != nil {
		return err
	}
	e.emit(vm.OpThrow, 0, 0, 0)
	e.popStack(1)
	return nil
}

func (e *emitter) emitBreak(n *phperr.PhpBreak) error {
	if n.Initial > 1 {
		return unsupportedf("multi-level break (break %d)", n.Initial)
	}
	loop := e.curLoop()
	if loop == nil {
		return unsupportedf("break outside loop (top-level fall-through?)")
	}
	pc := e.emit(vm.OpJmp, 0, 0, 0)
	loop.breakPCs = append(loop.breakPCs, pc)
	return nil
}

func (e *emitter) emitContinue(n *phperr.PhpContinue) error {
	if n.Initial > 1 {
		return unsupportedf("multi-level continue (continue %d)", n.Initial)
	}
	loop := e.curLoop()
	if loop == nil {
		return unsupportedf("continue outside loop")
	}
	pc := e.emit(vm.OpJmp, 0, 0, 0)
	loop.continuePCs = append(loop.continuePCs, pc)
	return nil
}

// stmtLoc returns the most appropriate source location for a statement
// node (for tick / error reporting). Returns nil if the node doesn't
// expose one in a way the emitter recognises.
func stmtLoc(node phpv.Runnable) *phpv.Loc {
	type locable interface {
		Loc() *phpv.Loc
	}
	if l, ok := node.(locable); ok {
		return l.Loc()
	}
	switch n := node.(type) {
	case ifNode:
		return n.IfLoc()
	case forNode:
		return n.ForLoc()
	case whileNode:
		return n.WhileLoc()
	case returnNode:
		return n.ReturnLoc()
	case throwNode:
		return n.ThrowLoc()
	case operatorNode:
		return n.OperatorLoc()
	case literalNode:
		return n.LiteralLoc()
	case variableNode:
		return n.VariableLoc()
	case funcCallNode:
		return n.FuncCallLoc()
	}
	return nil
}

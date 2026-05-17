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

type doWhileNode interface {
	DoWhileCond() phpv.Runnable
	DoWhileCode() phpv.Runnable
	DoWhileLoc() *phpv.Loc
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

	// NoDiscard-wrapped statement: AST-delegate so the warning
	// fires when the wrapped call's return value is discarded.
	if _, ok := node.(interface {
		NoDiscardInner() phpv.Runnable
	}); ok {
		raw, _ := node.(phpv.Runnable)
		idx := e.astIndex(raw)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
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

	// Switch is AST-delegated wholesale — break/continue inside the
	// switch are caught by the AST, while a return propagates out
	// via the PhpReturn error channel that OpTryFinally re-raises.
	if compiler.IsSwitchNode(node) {
		idx := e.astIndex(node)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	// `global $x;` / `static $y = …;` declarations bind a local to
	// outer storage. Delegate to the AST then resync the slot cache
	// so subsequent OP_LOAD_LOCAL reads see the bound value.
	if compiler.IsGlobalOrStaticDecl(node) {
		idx := e.astIndex(node)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	// Inline HTML (the text between `?>` and `<?php`): emit directly.
	if s, ok := compiler.InlineHtmlText(node); ok {
		idx := e.constIndex(s)
		e.emit(vm.OpInlineHtml, idx, 0, 0)
		return nil
	}

	// declare(strict_types=1): just call ctx.Global().SetStrictTypes(true).
	if compiler.IsDeclareStrictTypesNode(node) {
		e.emit(vm.OpSetStrictTypes, 0, 0, 0)
		return nil
	}

	// Statement-shaped delegations (declare ticks, top-level const,
	// enum register).
	if compiler.IsStmtAstDelegated(node) {
		idx := e.astIndex(node)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	// Statement-specific dispatch.
	switch n := node.(type) {
	case ifNode:
		return e.emitIf(n)
	case whileNode:
		return e.emitWhile(n)
	case doWhileNode:
		return e.emitDoWhile(n)
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

// emitDoWhile lowers `do { … } while (cond)` natively. Body runs at
// least once; continue lands on the cond check; break jumps past it.
func (e *emitter) emitDoWhile(n doWhileNode) error {
	loop := e.pushLoop()
	bodyStart := uint32(len(e.code))

	if err := e.emitStmt(n.DoWhileCode()); err != nil {
		return err
	}

	// continue target = cond check
	condStart := uint32(len(e.code))
	for _, pc := range loop.continuePCs {
		e.patchJump(pc, condStart)
	}

	if err := e.withSubexpr(func() error { return e.emitExpr(n.DoWhileCond()) }); err != nil {
		return err
	}
	// Jump back to body start if cond is true.
	jmpBack := e.emit(vm.OpJmpIfTrue, 0, 0, 0)
	e.popStack(1)
	e.patchJump(jmpBack, bodyStart)

	// Exit (cond false / break)
	exit := uint32(len(e.code))
	for _, pc := range loop.breakPCs {
		e.patchJump(pc, exit)
	}
	e.popLoop()
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

// isSimpleLocal returns true when r is a bare local variable node
// (the only foreach target shape the native foreach opcodes handle).
func isSimpleLocal(r phpv.Runnable) bool {
	_, ok := r.(variableNode)
	return ok
}

func (e *emitter) emitForeach(n foreachNode) error {
	// foreach-by-ref (`foreach($arr as &$v)`) and non-local targets
	// (`foreach($arr as $obj->prop => $val)`, list destructure, etc.)
	// are AST-delegated — the surrounding scope's body is flagged
	// slot-unsafe by IsSlotSafe so the hashtable stays authoritative
	// while the AST runs the loop.
	if n.ForeachIsRef() || !isSimpleLocal(n.ForeachValue()) || (n.ForeachKey() != nil && !isSimpleLocal(n.ForeachKey())) {
		raw, ok := any(n).(phpv.Runnable)
		if !ok {
			return unsupportedf("foreach delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	valNode := n.ForeachValue().(variableNode)
	valIdx := e.localIndex(valNode.VariableName())

	keyIdx := uint16(0xFFFF)
	if k := n.ForeachKey(); k != nil {
		kn := k.(variableNode)
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
	loop.isForeach = true
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
		// Coercion lives in runReturn.Run. AST-delegate the whole
		// statement — the PhpReturn it throws is caught by the VM
		// run loop's deferred wrapper just like OpRet.
		raw, ok := any(n).(phpv.Runnable)
		if !ok {
			return unsupportedf("typed return AST delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		// Unreachable on success (the AST throws PhpReturn), but the
		// emitter expects every path to terminate. Add a safety RET.
		e.emit(vm.OpRetNull, 0, 0, 0)
		return nil
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
	depth := int(n.Initial)
	if depth < 1 {
		depth = 1
	}
	if len(e.loops) < depth {
		if depth == 1 {
			return unsupportedf("break outside loop (top-level fall-through?)")
		}
		return unsupportedf("break %d with only %d enclosing loops", depth, len(e.loops))
	}
	// Unwind any foreach iterators that sit between the current
	// position and the target loop. Each OP_FOREACH_UNWIND pops one
	// entry from the iters stack — required for nested foreach.
	e.unwindForeachesAbove(depth - 1)
	loop := e.loops[len(e.loops)-depth]
	pc := e.emit(vm.OpJmp, 0, 0, 0)
	loop.breakPCs = append(loop.breakPCs, pc)
	return nil
}

func (e *emitter) emitContinue(n *phperr.PhpContinue) error {
	depth := int(n.Initial)
	if depth < 1 {
		depth = 1
	}
	if len(e.loops) < depth {
		if depth == 1 {
			return unsupportedf("continue outside loop")
		}
		return unsupportedf("continue %d with only %d enclosing loops", depth, len(e.loops))
	}
	// Same unwinding: drop foreach iterators for loops we're
	// skipping over (the inner foreach hasn't naturally completed,
	// so its iterator is still on the iters stack).
	e.unwindForeachesAbove(depth - 1)
	loop := e.loops[len(e.loops)-depth]
	pc := e.emit(vm.OpJmp, 0, 0, 0)
	loop.continuePCs = append(loop.continuePCs, pc)
	return nil
}

// unwindForeachesAbove emits an OP_FOREACH_UNWIND for each foreach
// loop in the top `skip` entries of e.loops. Used by multi-level
// break/continue to clean up the iters stack before jumping out.
func (e *emitter) unwindForeachesAbove(skip int) {
	n := len(e.loops)
	for i := 0; i < skip; i++ {
		if e.loops[n-1-i].isForeach {
			e.emit(vm.OpForeachUnwind, 0, 0, 0)
		}
	}
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

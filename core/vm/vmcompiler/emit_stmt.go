package vmcompiler

import (
	"fmt"

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

	// `#[NoDiscard]`-wrapped statement: bracket the inner stmt with
	// OP_NODISCARD_ENTER/EXIT. The prev in-context flag lives in a
	// synthetic local so the exit restores it on normal completion.
	// Re-enabled with B7 — the VM's OP_OBJECT_CALL path routes through
	// CallInstanceMethod → ctx.CallZVal → callZValImpl which sets
	// lastCallable for NoDiscard attributes the same way the AST does.
	if compiler.IsNoDiscardNode(node) {
		nd := node.(interface{ NoDiscardInner() phpv.Runnable })
		prevName := phpv.ZString(fmt.Sprintf("__nodiscard_prev_%d", e.nextSynthID()))
		prevIdx := e.localIndex(prevName)
		e.emit(vm.OpNoDiscardEnter, prevIdx, 0, 0)
		if err := e.emitStmt(nd.NoDiscardInner()); err != nil {
			return err
		}
		e.emit(vm.OpNoDiscardExit, prevIdx, 0, 0)
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

	if sn, ok := node.(compiler.SwitchNode); ok {
		return e.emitSwitch(sn)
	}

	// `global $x;` declaration: native lowering per-entry.
	if compiler.IsGlobalDecl(node) {
		for _, ent := range compiler.GlobalEntries(node) {
			if dyn := ent.GlobalDynamic(); dyn != nil {
				if err := e.withSubexpr(func() error { return e.emitExpr(dyn) }); err != nil {
					return err
				}
			} else {
				idx := e.constIndex(ent.GlobalStatic())
				e.emit(vm.OpLoadConst, idx, 0, 0)
				e.pushStack(1)
			}
			e.emit(vm.OpGlobalBind, 0, 0, 0)
			e.popStack(1)
		}
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	// `static $y = …;` declaration: native binding via OP_STATIC_VAR_BIND,
	// which calls the shared compiler.BindStaticVars helper to handle
	// per-closure / per-class / global storage.
	if compiler.IsStaticVarDecl(node) {
		idx := e.astIndex(node)
		e.emit(vm.OpStaticVarBind, idx, 0, 0)
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

	// declare(ticks=N) { body }: emit body, OP_CALL_TICK_FUNCTIONS every N stmts.
	if dt, ok := node.(interface {
		DeclareTicksBody() phpv.Runnable
		DeclareTicksN() int64
	}); ok {
		body := dt.DeclareTicksBody()
		if body == nil {
			return nil
		}
		n := dt.DeclareTicksN()
		if n < 1 {
			n = 1
		}
		// If body is a Runnables, intersperse tick calls.
		if stmts, ok := body.(phpv.Runnables); ok {
			var count int64
			for _, s := range stmts {
				if err := e.emitStmt(s); err != nil {
					return err
				}
				count++
				if count%n == 0 {
					e.emit(vm.OpCallTickFunctions, 0, 0, 0)
				}
			}
			return nil
		}
		// Single-statement body.
		if err := e.emitStmt(body); err != nil {
			return err
		}
		e.emit(vm.OpCallTickFunctions, 0, 0, 0)
		return nil
	}

	// `const NAME = expr;` top-level definition: emit the value expr
	// natively, then a single OP_DEFINE_CONST that reads the static
	// metadata (name / attrs / loc) from the AST node slot.
	if compiler.IsTopLevelConst(node) {
		_, val, _, _ := compiler.TopLevelConstParts(node)
		if err := e.withSubexpr(func() error { return e.emitExpr(val) }); err != nil {
			return err
		}
		idx := e.astIndex(node)
		e.emit(vm.OpDefineConst, idx, 0, 0)
		e.popStack(1)
		return nil
	}

	// `enum Foo { … }` declaration: native dispatch via OP_REGISTER_ENUM.
	// The shared compiler.RegisterEnum helper handles class registration,
	// pre/post-compile validation, and Compile() in one go.
	if compiler.IsEnumRegisterNode(node) {
		idx := e.astIndex(node)
		e.emit(vm.OpRegisterEnum, idx, 0, 0)
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
		return e.emitTry(n)
	case *phperr.PhpBreak:
		return e.emitBreak(n)
	case *phperr.PhpContinue:
		return e.emitContinue(n)
	}

	// Bare-variable statement (`$x;`): PHP's compiler drops this — no
	// FETCH_R is emitted, so no "Undefined variable" warning fires.
	// The AST runner's runVariable.Run replicates that via a write=true
	// short-circuit when Parent is a Runnables. The VM has no parent
	// concept, so detect and skip it here directly.
	if _, ok := node.(variableNode); ok {
		return nil
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

// emitSwitch lowers `switch (cond) { case X: …; default: …; … }`.
//
// Layout:
//
//	emit cond → push
//	STORE_LOCAL __switch_cond_N      # save cond once
//	# Dispatch phase — for each non-default block:
//	LOAD_LOCAL __switch_cond_N
//	emit case_i                       # may evaluate side-effects per
//	CMP_EQ                            # PHP's "stop at first match" order
//	JMP_IF_TRUE L_body[i]
//	# (next case checked similarly; falls through naturally)
//	JMP L_default_or_break            # no match
//	# Bodies in source order (allows fall-through):
//	L_body[0]: emit blocks[0].code
//	L_body[1]: emit blocks[1].code
//	…
//	L_break: (== loop break/continue target)
//
// Break and continue with depth 1 inside the switch jump to L_break.
// Multi-level break/continue propagates to the enclosing loop via
// the loops stack (switch counts as one level there).
func (e *emitter) emitSwitch(n compiler.SwitchNode) error {
	if err := e.withSubexpr(func() error { return e.emitExpr(n.SwitchCond()) }); err != nil {
		return err
	}
	condName := phpv.ZString(fmt.Sprintf("__switch_cond_%d", e.nextSynthID()))
	condIdx := e.localIndex(condName)
	e.emit(vm.OpStoreLocal, condIdx, 0, 0)
	e.popStack(1)

	loop := e.pushLoop()
	defer e.popLoop()

	blocks := n.SwitchBlocks()
	matchJumpPCs := make([]uint32, len(blocks))
	defaultIdx := -1
	for i, bl := range blocks {
		if bl.SwitchBlockCond() == nil {
			defaultIdx = i
			matchJumpPCs[i] = 0xFFFFFFFF
			continue
		}
		e.emit(vm.OpLoadLocal, condIdx, 0, 0)
		e.pushStack(1)
		if err := e.withSubexpr(func() error { return e.emitExpr(bl.SwitchBlockCond()) }); err != nil {
			return err
		}
		e.emit(vm.OpCmpEq, 0, 0, 0)
		e.popStack(1) // pops 2, pushes 1
		matchJumpPCs[i] = e.emit(vm.OpJmpIfTrue, 0, 0, 0)
		e.popStack(1) // bool consumed
	}

	// No-match jump: either to default body or to break target.
	noMatchJmp := e.emit(vm.OpJmp, 0, 0, 0)
	if defaultIdx == -1 {
		loop.breakPCs = append(loop.breakPCs, noMatchJmp)
	}

	// Bodies in source order — fall-through is natural.
	bodyStartPCs := make([]uint32, len(blocks))
	for i, bl := range blocks {
		bodyStartPCs[i] = uint32(len(e.code))
		if err := e.emitStmt(bl.SwitchBlockCode()); err != nil {
			return err
		}
	}

	// Patch dispatch jumps to their respective body PCs.
	for i, pc := range matchJumpPCs {
		if pc == 0xFFFFFFFF {
			continue
		}
		e.patchJump(pc, bodyStartPCs[i])
	}
	if defaultIdx >= 0 {
		e.patchJump(noMatchJmp, bodyStartPCs[defaultIdx])
	}

	// L_break == current PC.
	brk := uint32(len(e.code))
	for _, pc := range loop.breakPCs {
		e.patchJump(pc, brk)
	}
	// PHP: `continue` with depth 1 inside switch acts as break.
	for _, pc := range loop.continuePCs {
		e.patchJump(pc, brk)
	}
	return nil
}

func (e *emitter) emitForeach(n foreachNode) error {
	// foreach-by-ref (`foreach($arr as &$v)`) still AST-delegates — the
	// iterator needs to yield refs and the helper that snapshots arrays
	// uses different CoW semantics. The slot-unsafe flag on the
	// enclosing function (set by IsSlotSafe) keeps the hashtable
	// authoritative for the AST loop. Non-simple-local targets WITHOUT
	// by-ref take the native path below.
	if n.ForeachIsRef() {
		raw, ok := any(n).(phpv.Runnable)
		if !ok {
			return unsupportedf("foreach by-ref delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpTryFinally, idx, 0, 0)
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	valIsLocal := isSimpleLocal(n.ForeachValue())
	keyExpr := n.ForeachKey()
	keyIsLocal := keyExpr == nil || isSimpleLocal(keyExpr)

	// Compute the operand for OP_FOREACH_STEP / OP_FOREACH_STEP_PUSH.
	// When both targets are bare locals, the in-place store path (OP_FOREACH_STEP)
	// avoids the stack push/pop entirely.
	valLocalIdx := uint16(0xFFFF)
	keyLocalIdx := uint16(0xFFFF)
	if valIsLocal {
		valLocalIdx = e.localIndex(n.ForeachValue().(variableNode).VariableName())
	}
	if keyExpr != nil && keyIsLocal {
		keyLocalIdx = e.localIndex(keyExpr.(variableNode).VariableName())
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
	useNative := valIsLocal && keyIsLocal
	var stepPC uint32
	if useNative {
		stepPC = e.emit(vm.OpForeachStep, valLocalIdx, keyLocalIdx, 0)
	} else {
		// Push key (if present) and value onto stack.
		keyFlag := uint16(0)
		if keyExpr != nil {
			keyFlag = 1
		}
		stepPC = e.emit(vm.OpForeachStepPush, keyFlag, 0, 0)
		if keyExpr != nil {
			e.pushStack(1) // key
		}
		e.pushStack(1) // value

		// Pop value off the stack first, write to value target.
		if err := e.emitForeachTargetWrite(n.ForeachValue()); err != nil {
			return err
		}
		// Then pop key (if present) and write to key target.
		if keyExpr != nil {
			if err := e.emitForeachTargetWrite(keyExpr); err != nil {
				return err
			}
		}
	}

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

// emitForeachTargetWrite pops a value off the stack and writes it to
// the foreach target node `target`. Used when the value/key isn't a
// bare local — the runtime push the value, this code consumes it.
//
//   - destructure (`[$a, $b]`) → OP_DESTRUCTURE_ASSIGN
//   - any other Writable shape → OP_ASSIGN_WRITABLE delegating to the
//     AST node's WriteValue (handles obj prop, array element, static
//     prop, dynamic-name variable, etc.)
//   - simple local → OP_STORE_LOCAL (only happens when this is called
//     for the key side and the value side forced the push path)
func (e *emitter) emitForeachTargetWrite(target phpv.Runnable) error {
	if isSimpleLocal(target) {
		idx := e.localIndex(target.(variableNode).VariableName())
		e.emit(vm.OpStoreLocal, idx, 0, 0)
		e.popStack(1)
		return nil
	}
	if compiler.IsDestructureTarget(target) {
		idx := e.astIndex(target)
		// flags = 0 → stmt-context (drop the value, don't push it back).
		e.emit(vm.OpDestructureAssign, idx, 0, 0)
		e.popStack(1)
		// Locals may have changed via destructure's WriteValue.
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	// Generic Writable (object prop, array access, static prop, …).
	idx := e.astIndex(target)
	e.emit(vm.OpAssignWritable, idx, 0, 0)
	e.popStack(1)
	// Conservatively refresh — the WriteValue may have set a local
	// through OffsetSet that the slot cache doesn't see.
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	return nil
}

func (e *emitter) emitReturn(n returnNode) error {
	v := n.ReturnValue()
	if n.ReturnHasTypeHint() {
		// Native typed-return is safe when the return value isn't a
		// property/array-access/static-prop — those shapes trigger
		// writeContext setup in runReturn.Run when the enclosing
		// function returns by reference, which the native emit can't
		// reproduce without knowing the by-ref flag at emit time.
		raw, _ := any(n).(phpv.Runnable)
		if v == nil || compiler.ReturnValueIsRefTarget(v) {
			if raw == nil {
				return unsupportedf("typed return AST delegation: cannot retrieve raw Runnable")
			}
			idx := e.astIndex(raw)
			e.emit(vm.OpTryFinally, idx, 0, 0)
			e.emit(vm.OpRetNull, 0, 0, 0)
			return nil
		}
		// Safe shape: emit value natively, coerce, then OP_RET.
		if err := e.withSubexpr(func() error { return e.emitExpr(v) }); err != nil {
			return err
		}
		if raw != nil {
			idx := e.astIndex(raw)
			e.emit(vm.OpCoerceReturn, idx, 0, 0)
		}
		e.emit(vm.OpRet, 0, 0, 0)
		e.popStack(1)
		return nil
	}
	if v != nil {
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
	// Cross-finally break: target loop sits below the innermost active
	// finally body. The native lowering parks pending control via
	// OpRet/OpFinallyEnd; break doesn't route through that machinery
	// yet, so fall back to the AST runner for the whole function.
	if n := len(e.finallyLoopDepths); n > 0 {
		targetIdx := len(e.loops) - depth
		if targetIdx < e.finallyLoopDepths[n-1] {
			return unsupportedf("break across try-finally")
		}
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
	if n := len(e.finallyLoopDepths); n > 0 {
		targetIdx := len(e.loops) - depth
		if targetIdx < e.finallyLoopDepths[n-1] {
			return unsupportedf("continue across try-finally")
		}
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

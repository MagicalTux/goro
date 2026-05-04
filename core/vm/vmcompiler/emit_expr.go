package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
	"github.com/KarpelesLab/goro/core/vm"
)

// --- accessor interfaces matching core/compiler exports ----------------

type literalNode interface {
	LiteralVal() phpv.Val
	LiteralLoc() *phpv.Loc
}

type variableNode interface {
	VariableName() phpv.ZString
	VariableLoc() *phpv.Loc
}

type constantNode interface {
	ConstantName() string
	ConstantLoc() *phpv.Loc
}

type operatorNode interface {
	OperatorOp() tokenizer.ItemType
	OperatorA() phpv.Runnable
	OperatorB() phpv.Runnable
	OperatorLoc() *phpv.Loc
	OperatorIsWrite() bool
	OperatorIsCompound() bool
}

// --- expression emission ----------------------------------------------

// emitExpr emits bytecode that pushes the value of node onto the VM
// stack. Returns ErrUnsupported (wrapped) for any node type the
// emitter doesn't lower natively — the caller (Compile) propagates
// this so the surrounding function body falls back to the AST.
func (e *emitter) emitExpr(node phpv.Runnable) error {
	if node == nil {
		// `return;` lowers to `return null;`. Emit a literal null.
		e.emit(vm.OpLoadNull, 0, 0, 0)
		e.pushStack(1)
		return nil
	}

	switch n := node.(type) {
	case literalNode:
		return e.emitLiteral(n)
	case variableNode:
		return e.emitVariableRead(n)
	case constantNode:
		return e.emitConstant(n)
	case operatorNode:
		return e.emitOperator(n)
	case funcCallNode:
		return e.emitFunctionCall(n)
	case arrayLiteralNode:
		return e.emitArrayLiteral(n)
	case arrayAccessNode:
		return e.emitArrayAccessRead(n)
	case newObjectNode:
		return e.emitNewObject(n)
	case objectVarNode:
		return e.emitObjectVarRead(n)
	case objectFuncNode:
		return e.emitObjectFuncCall(n)
	}

	// Runnables (slice of statements) appears as an expression rarely
	// (e.g. inside a parenthesised group). Defer to a single-result
	// helper so we can handle it uniformly.
	if rs, ok := node.(phpv.Runnables); ok && len(rs) == 1 {
		return e.emitExpr(rs[0])
	}

	return unsupportedf("emitExpr: %T", node)
}

func (e *emitter) emitLiteral(n literalNode) error {
	v := n.LiteralVal()
	switch x := v.(type) {
	case phpv.ZNull:
		e.emit(vm.OpLoadNull, 0, 0, 0)
		e.pushStack(1)
		return nil
	case phpv.ZBool:
		if bool(x) {
			e.emit(vm.OpLoadTrue, 0, 0, 0)
		} else {
			e.emit(vm.OpLoadFalse, 0, 0, 0)
		}
		e.pushStack(1)
		return nil
	case phpv.ZInt, phpv.ZFloat, phpv.ZString:
		idx := e.constIndex(v)
		e.emit(vm.OpLoadConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	return unsupportedf("emitLiteral: %T", v)
}

func (e *emitter) emitConstant(n constantNode) error {
	// Only the three case-insensitive literal constants are handled
	// directly. Everything else (PHP_INT_MAX, MYAPP_FOO, …) requires
	// runtime ConstantGet which we'll add later.
	switch lower := lowerASCII(n.ConstantName()); lower {
	case "null":
		e.emit(vm.OpLoadNull, 0, 0, 0)
		e.pushStack(1)
		return nil
	case "true":
		e.emit(vm.OpLoadTrue, 0, 0, 0)
		e.pushStack(1)
		return nil
	case "false":
		e.emit(vm.OpLoadFalse, 0, 0, 0)
		e.pushStack(1)
		return nil
	}
	return unsupportedf("user-defined constant %q", n.ConstantName())
}

// lowerASCII lowercases an ASCII identifier without allocating for
// already-lowercase inputs. PHP's true/false/null match
// case-insensitively.
func lowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

func (e *emitter) emitVariableRead(n variableNode) error {
	idx := e.localIndex(n.VariableName())
	// Always use OpLoadLocal here — the AST distinguishes write/read
	// context per parent node, but for the MVP scope (variables only
	// appearing as RHS reads or LHS write targets) we always want the
	// "warn on undefined" behaviour for reads. The LHS case is handled
	// separately in emitOperator (which never goes through this path
	// for a write target).
	e.emit(vm.OpLoadLocalOrWarn, idx, 0, 0)
	e.pushStack(1)
	return nil
}

func (e *emitter) emitOperator(n operatorNode) error {
	op := n.OperatorOp()

	// ++ / --
	if op == tokenizer.T_INC || op == tokenizer.T_DEC {
		return e.emitIncDec(n, op == tokenizer.T_INC)
	}

	// Plain assignment `=`
	if n.OperatorIsWrite() && !n.OperatorIsCompound() {
		return e.emitAssign(n)
	}

	// Compound assignment (+=, -=, .=, …)
	if n.OperatorIsCompound() {
		return e.emitCompoundAssign(n, op)
	}

	// Logical short-circuit ops
	switch op {
	case tokenizer.T_BOOLEAN_AND, tokenizer.T_LOGICAL_AND:
		return e.emitShortCircuit(n, false)
	case tokenizer.T_BOOLEAN_OR, tokenizer.T_LOGICAL_OR:
		return e.emitShortCircuit(n, true)
	case tokenizer.T_COALESCE:
		return e.emitCoalesce(n)
	}

	// Unary ops where a == nil and only b is the operand.
	a := n.OperatorA()
	b := n.OperatorB()
	if a == nil && b != nil {
		return e.emitUnary(b, op)
	}

	// Binary ops.
	if a != nil && b != nil {
		return e.emitBinary(a, b, op)
	}

	return unsupportedf("emitOperator: op=%v a=%v b=%v", op, a == nil, b == nil)
}

// withSubexpr runs fn with stmtCtx forced to false, so that nested
// expressions know their result is consumed and don't insert their own
// drop. The previous flag is restored on return.
func (e *emitter) withSubexpr(fn func() error) error {
	prev := e.stmtCtx
	e.stmtCtx = false
	defer func() { e.stmtCtx = prev }()
	return fn()
}

func (e *emitter) emitBinary(a, b phpv.Runnable, op tokenizer.ItemType) error {
	bop, ok := binaryOpcode(op)
	if !ok {
		return unsupportedf("binary op %v", op)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(b) }); err != nil {
		return err
	}
	e.emit(bop, 0, 0, 0)
	e.popStack(1) // pop two, push one
	return nil
}

func (e *emitter) emitUnary(b phpv.Runnable, op tokenizer.ItemType) error {
	uop, ok := unaryOpcode(op)
	if !ok {
		return unsupportedf("unary op %v", op)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(b) }); err != nil {
		return err
	}
	e.emit(uop, 0, 0, 0)
	// pop 1 push 1 — net zero stack delta
	return nil
}

func (e *emitter) emitAssign(n operatorNode) error {
	// Plain `=` to a simple local variable.
	if v, ok := n.OperatorA().(variableNode); ok {
		stmtCtx := e.stmtCtx
		if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
			return err
		}
		idx := e.localIndex(v.VariableName())
		if stmtCtx {
			e.emit(vm.OpStoreLocal, idx, 0, 0)
			e.popStack(1)
		} else {
			e.emit(vm.OpStoreLocalKeep, idx, 0, 0)
		}
		return nil
	}
	// `$local[k] = v` and `$local[] = v` (statement context only).
	if aa, ok := n.OperatorA().(arrayAccessNode); ok {
		stmtCtx := e.stmtCtx
		return e.emitArrayAssignToLocal(aa, n.OperatorB(), stmtCtx)
	}
	return unsupportedf("plain `=` to non-variable target %T", n.OperatorA())
}

func (e *emitter) emitCompoundAssign(n operatorNode, op tokenizer.ItemType) error {
	lhs, ok := n.OperatorA().(variableNode)
	if !ok {
		return unsupportedf("compound assign to non-variable target %T", n.OperatorA())
	}
	stmtCtx := e.stmtCtx
	if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
		return err
	}
	idx := e.localIndex(lhs.VariableName())
	// The MVP version of OP_OP_ASSIGN_LOCAL doesn't push a result;
	// the consumer is the statement-context for-step or expression
	// statement. If a non-statement context (e.g. `($x += 1) * 2`)
	// hits this path we fall back to AST since we'd need to push the
	// post-modification value too.
	if !stmtCtx {
		return unsupportedf("compound assign as expression (non-statement context)")
	}
	e.emit(vm.OpOpAssignLocal, idx, uint16(op), 0)
	e.popStack(1)
	return nil
}

func (e *emitter) emitIncDec(n operatorNode, inc bool) error {
	// Determine target: postfix has a set, prefix has b set.
	var target phpv.Runnable
	post := false
	if a := n.OperatorA(); a != nil {
		target = a
		post = true
	} else {
		target = n.OperatorB()
	}
	tv, ok := target.(variableNode)
	if !ok {
		return unsupportedf("++/-- on non-variable target %T", target)
	}
	idx := e.localIndex(tv.VariableName())

	switch {
	case post && inc:
		e.emit(vm.OpPostIncLocal, idx, 0, 0)
	case post && !inc:
		e.emit(vm.OpPostDecLocal, idx, 0, 0)
	case !post && inc:
		e.emit(vm.OpIncLocal, idx, 0, 0)
	case !post && !inc:
		e.emit(vm.OpDecLocal, idx, 0, 0)
	}
	e.pushStack(1)

	// In statement context the pushed value is unused — drop it.
	if e.stmtCtx {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	return nil
}

// emitCoalesce emits `a ?? b` with PHP's "set and non-null" semantics:
// undefined variables on the LHS don't warn; if the LHS evaluates to
// non-null, RHS is skipped.
//
// To suppress the "Undefined variable" warning on a simple variable
// LHS, we emit OP_LOAD_LOCAL (the no-warn form) directly when the LHS
// is a variableNode. For other LHS expressions we fall back to AST
// since the AST runArrayAccess / runObjectVar have writeContext-style
// suppression that's hard to replicate here.
func (e *emitter) emitCoalesce(n operatorNode) error {
	a := n.OperatorA()
	b := n.OperatorB()
	if a == nil || b == nil {
		return unsupportedf("coalesce missing operand")
	}

	// LHS evaluation — for a simple variable, use the silent load.
	if v, ok := a.(variableNode); ok {
		idx := e.localIndex(v.VariableName())
		e.emit(vm.OpLoadLocal, idx, 0, 0)
		e.pushStack(1)
	} else {
		// For now only the variable form is supported. Other LHS
		// shapes (array elements, object props) require write-context
		// suppression that the AST handles internally.
		return unsupportedf("coalesce LHS %T (only $local supported)", a)
	}

	// If LHS is non-null, jump past the RHS — the LHS value stays on
	// the stack as the result.
	jpc := e.emit(vm.OpJmpIfNotNullPeek, 0, 0, 0)

	// Else: drop the null LHS, evaluate RHS, fall through.
	e.emit(vm.OpPop, 0, 0, 0)
	e.popStack(1)
	if err := e.withSubexpr(func() error { return e.emitExpr(b) }); err != nil {
		return err
	}

	// End label.
	e.patchJump(jpc, uint32(len(e.code)))
	return nil
}

// emitShortCircuit emits `a && b` (orMode=false) or `a || b`
// (orMode=true) preserving short-circuit semantics. Both branches
// leave a single bool on the stack.
func (e *emitter) emitShortCircuit(n operatorNode, orMode bool) error {
	if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorA()) }); err != nil {
		return err
	}
	// Cast to bool now so the result is always a bool regardless of
	// short-circuit branch.
	e.emit(vm.OpBool, 0, 0, 0)

	// If ||  → if top is true, skip b and keep top.
	// If && → if top is false, skip b and keep top.
	var jmpOp vm.Op
	if orMode {
		jmpOp = vm.OpJmpIfTruePeek
	} else {
		jmpOp = vm.OpJmpIfFalsePeek
	}
	jpc := e.emit(jmpOp, 0, 0, 0)

	// Otherwise, drop the LHS bool and evaluate the RHS.
	e.emit(vm.OpPop, 0, 0, 0)
	e.popStack(1)
	if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
		return err
	}
	e.emit(vm.OpBool, 0, 0, 0)

	// Patch the short-circuit jump to land here.
	e.patchJump(jpc, uint32(len(e.code)))
	return nil
}

// --- op-table -----------------------------------------------------------

func binaryOpcode(op tokenizer.ItemType) (vm.Op, bool) {
	switch op {
	case tokenizer.Rune('+'):
		return vm.OpAdd, true
	case tokenizer.Rune('-'):
		return vm.OpSub, true
	case tokenizer.Rune('*'):
		return vm.OpMul, true
	case tokenizer.Rune('/'):
		return vm.OpDiv, true
	case tokenizer.Rune('%'):
		return vm.OpMod, true
	case tokenizer.T_POW:
		return vm.OpPow, true
	case tokenizer.Rune('&'):
		return vm.OpBitAnd, true
	case tokenizer.Rune('|'):
		return vm.OpBitOr, true
	case tokenizer.Rune('^'):
		return vm.OpBitXor, true
	case tokenizer.T_SL:
		return vm.OpShl, true
	case tokenizer.T_SR:
		return vm.OpShr, true
	case tokenizer.Rune('.'):
		return vm.OpConcat, true
	case tokenizer.T_IS_EQUAL:
		return vm.OpCmpEq, true
	case tokenizer.T_IS_NOT_EQUAL:
		return vm.OpCmpNe, true
	case tokenizer.T_IS_IDENTICAL:
		return vm.OpCmpId, true
	case tokenizer.T_IS_NOT_IDENTICAL:
		return vm.OpCmpNid, true
	case tokenizer.Rune('<'):
		return vm.OpCmpLt, true
	case tokenizer.T_IS_SMALLER_OR_EQUAL:
		return vm.OpCmpLe, true
	case tokenizer.Rune('>'):
		return vm.OpCmpGt, true
	case tokenizer.T_IS_GREATER_OR_EQUAL:
		return vm.OpCmpGe, true
	case tokenizer.T_SPACESHIP:
		return vm.OpCmpSpaceship, true
	}
	return 0, false
}

func unaryOpcode(op tokenizer.ItemType) (vm.Op, bool) {
	switch op {
	case tokenizer.Rune('-'):
		return vm.OpNeg, true
	case tokenizer.Rune('~'):
		return vm.OpBitNot, true
	case tokenizer.Rune('!'):
		return vm.OpNot, true
	}
	return 0, false
}

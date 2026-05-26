package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
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

// concatNode matches compiler.runConcat — string interpolation.
type concatNode interface {
	ConcatParts() []phpv.Runnable
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
	// Strip `(expr)` wrappers — they affect by-ref param Notices
	// only, which the VM doesn't lower piecewise anyway.
	node = compiler.UnwrapParens(node)

	if compiler.IsClosureNode(node) {
		// *ZClosure as expression: emit OP_MAKE_CLOSURE which runs
		// the AST node's Run method (it does dup + capture + Spawn
		// for inline closures, RegisterFunction for named ones).
		idx := e.closureIndex(node)
		e.emit(vm.OpMakeClosure, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsIssetOrEmptyNode(node) {
		// All-simple-local isset/empty lowers natively. Complex forms
		// (array access, object prop, dyn names, …) keep the AST path.
		if compiler.IssetEmptyAllSupported(node) {
			return e.emitIssetEmptySupported(node)
		}
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsAnonymousClassNode(node) {
		// `new class { … }(args)` — for the simple-arg case (no
		// CompoundWritable args) lower natively via OP_NEW_ANON_CLASS;
		// otherwise fall back to AST so the by-ref auto-vivification
		// dance in evalConstructorArgs still runs.
		if compiler.AnonClassHasWritableArg(node) {
			idx := e.astIndex(node)
			e.emit(vm.OpClassConst, idx, 0, 0)
			e.pushStack(1)
			return nil
		}
		args := compiler.AnonymousClassArgs(node)
		for _, a := range args {
			if err := e.withSubexpr(func() error { return e.emitExpr(a) }); err != nil {
				return err
			}
		}
		idx := e.astIndex(node)
		e.emit(vm.OpNewAnonClass, idx, uint16(len(args)), 0)
		e.popStack(len(args)) // pops argc args, pushes 1 obj → net -argc+1
		e.pushStack(1)
		return nil
	}
	if mn, ok := node.(compiler.MatchNode); ok {
		return e.emitMatch(mn)
	}
	if compiler.IsVariableRef(node) {
		// `$$name` / `${$expr}` read — emit name expr, then
		// OP_VAR_VAR_READ with the parent-context-aware warn flag.
		writeCtx := compiler.VarVarReadIsWriteParent(node)
		if err := e.withSubexpr(func() error { return e.emitExpr(compiler.VarVarNameExpr(node)) }); err != nil {
			return err
		}
		var aFlags uint16
		if !writeCtx {
			aFlags |= 1 // warn flag
		}
		e.emit(vm.OpVarVarRead, aFlags, 0, 0)
		// pop name + push result → net 0
		return nil
	}
	if compiler.IsInstanceOfNode(node) {
		return e.emitInstanceOf(node)
	}
	if compiler.IsBasicCloneNode(node) {
		return e.emitClone(node)
	}
	if compiler.IsClassNameOfNode(node) {
		return e.emitClassNameOf(node)
	}
	if compiler.IsVoidCastNode(node) {
		return e.emitVoidCast(node)
	}
	if compiler.IsFirstClassCallableNode(node) {
		return e.emitFirstClassCallable(node)
	}
	if compiler.IsFirstClassCloneCallableNode(node) {
		e.emit(vm.OpFirstClassClone, 0, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsFirstClassMethodCallableNode(node) {
		return e.emitMethodFirstClass(node)
	}
	if compiler.IsFirstClassDynMethodCallableNode(node) {
		return e.emitDynMethodFirstClass(node)
	}
	if compiler.IsObjectDynVarReadNode(node) {
		return e.emitObjectDynGet(node)
	}
	if compiler.IsClassDynConstNode(node) {
		return e.emitClassDynConst(node)
	}
	if compiler.IsClassStaticVarReadNode(node) {
		return e.emitClassStaticGet(node)
	}
	if compiler.IsClassStaticObjRefNode(node) {
		return e.emitClassStaticObjRef(node)
	}
	if compiler.IsClassStaticDynVarReadNode(node) {
		return e.emitClassStaticDynGet(node)
	}
	if compiler.IsRefNode(node) {
		// `&$expr` reference creation — dedicated OP_CREATE_REF that
		// calls compiler.EvalCreateRef (which handles per-target-shape
		// semantics). No generic OpClassConst delegation.
		idx := e.astIndex(node)
		e.emit(vm.OpCreateRef, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsCloneExtNode(node) {
		// Extended `clone($x, $with)` / `clone(...$arr)` (PHP 8.5+)
		// — dedicated OP_CLONE_EXT calls compiler.EvalCloneExt.
		idx := e.astIndex(node)
		e.emit(vm.OpCloneExt, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsObjectDynFuncNode(node) {
		// `$obj->{$expr}(args)` / `Foo::{$expr}(args)` dyn-method-name
		// call. Stage 2 lowering: emit receiver + name-producing
		// expression natively to the stack; OP_OBJECT_DYN_CALL pops
		// both and calls EvalObjectDynFuncWithEvaluated, which then
		// dispatches static / instance / __call / __callStatic and
		// runs ctx.Call to bind by-ref / named / spread args.
		stmtCtx := e.stmtCtx
		recv := compiler.ObjectDynFuncReceiver(node)
		name := compiler.ObjectDynFuncNameExpr(node)
		if err := e.withSubexpr(func() error { return e.emitExpr(recv) }); err != nil {
			return err
		}
		if err := e.withSubexpr(func() error { return e.emitExpr(name) }); err != nil {
			return err
		}
		idx := e.astIndex(node)
		e.emit(vm.OpObjectDynCall, idx, 0, 0)
		// Pops 2 (recv + name), pushes 1 result. Net delta: -1.
		e.popStack(1)
		if stmtCtx {
			e.emit(vm.OpPop, 0, 0, 0)
			e.popStack(1)
		}
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	if compiler.IsValueExprAstDelegated(node) {
		// Common value-expression types we delegate wholesale
		// (clone, instanceof, void-cast, first-class callables, …).
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsUnsetNode(node) {
		// Native emit for the simple shapes: simple-variable args and
		// array-access on a simple-local container. Other shapes
		// (object prop, static prop, dyn names, nested array access)
		// still AST-delegate via the AST WriteValue pipeline.
		stmtCtx := e.stmtCtx
		if compiler.UnsetAllSupported(node) {
			for _, a := range compiler.UnsetArgs(node) {
				if compiler.IsSimpleVariable(a) {
					idx := e.localIndex(compiler.SimpleVariableName(a))
					e.emit(vm.OpUnsetLocal, idx, 0, 0)
					continue
				}
				if ov, ok := a.(objectVarNode); ok {
					// $obj->prop unset: emit receiver, then OP_UNSET_OBJ_PROP
					// with the property name in the const pool.
					if err := e.withSubexpr(func() error { return e.emitExpr(ov.ObjectVarReceiver()) }); err != nil {
						return err
					}
					idx := e.constIndex(ov.ObjectVarName())
					e.emit(vm.OpUnsetObjProp, idx, 0, 0)
					e.popStack(1)
					continue
				}
				// Array access on a simple-local container.
				cont, off := compiler.ArrayAccessParts(a)
				if err := e.withSubexpr(func() error { return e.emitExpr(cont) }); err != nil {
					return err
				}
				if err := e.withSubexpr(func() error { return e.emitExpr(off) }); err != nil {
					return err
				}
				e.emit(vm.OpUnsetDim, 0, 0, 0)
				e.popStack(2)
			}
			if !stmtCtx {
				e.emit(vm.OpLoadNull, 0, 0, 0)
				e.pushStack(1)
			}
			return nil
		}
		idx := e.astIndex(node)
		if stmtCtx {
			e.emit(vm.OpTryFinally, idx, 0, 0)
		} else {
			e.emit(vm.OpClassConst, idx, 0, 0)
			e.pushStack(1)
		}
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}

	switch n := node.(type) {
	case ifNode:
		// Ternary `cond ? a : b` as a value-producing expression
		// (the statement-level form is handled in emitStmt).
		return e.emitTernary(n)
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
	case funcCallRefNode:
		return e.emitFunctionCallRef(n)
	case arrayLiteralNode:
		return e.emitArrayLiteral(n)
	case arrayAccessNode:
		return e.emitArrayAccessRead(n)
	case concatNode:
		return e.emitConcat(n)
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

// emitConcat lowers `"foo $bar baz"`-style interpolation to N-1
// OP_CONCAT chains. The empty case produces an empty string.
func (e *emitter) emitConcat(n concatNode) error {
	parts := n.ConcatParts()
	if len(parts) == 0 {
		idx := e.constIndex(phpv.ZString(""))
		e.emit(vm.OpLoadConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(parts[0]) }); err != nil {
		return err
	}
	for _, p := range parts[1:] {
		if err := e.withSubexpr(func() error { return e.emitExpr(p) }); err != nil {
			return err
		}
		e.emit(vm.OpConcat, 0, 0, 0)
		e.popStack(1) // 2 pop, 1 push
	}
	return nil
}

func (e *emitter) emitConstant(n constantNode) error {
	// Fast path: case-insensitive literal constants null/true/false.
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
	// All other constants (PHP_INT_MAX, namespaced consts, user-defined,
	// …) go through OP_LOAD_CONSTANT_BY_NAME, which calls the shared
	// compiler.LookupConstant helper.
	raw, ok := any(n).(phpv.Runnable)
	if !ok {
		return unsupportedf("user-defined constant %q: cannot retrieve raw Runnable", n.ConstantName())
	}
	cName, noFallback, cLoc := compiler.ConstantNameAndFlags(raw)
	idx := e.constIndex(phpv.ZString(cName))
	var bFlags uint16
	if noFallback {
		bFlags |= 1
	}
	// Record the constant's own source location so an "Undefined
	// constant" Error reports the exact line (LocAt(pc) reads this).
	if cLoc != nil {
		e.recordLoc(cLoc)
	}
	e.emit(vm.OpLoadConstantByName, idx, bFlags, 0)
	e.pushStack(1)
	return nil
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

// instanceofNode is implemented by *compiler.runInstanceOf and exposes
// the LHS value and either a static class name or a dynamic-class
// expression on the RHS.
type instanceofNode interface {
	InstanceOfValue() phpv.Runnable
	InstanceOfStaticClass() phpv.ZString
	InstanceOfClassVar() phpv.Runnable
}

func (e *emitter) emitInstanceOf(node phpv.Runnable) error {
	n, ok := node.(instanceofNode)
	if !ok {
		return unsupportedf("instanceof: node %T doesn't expose accessors", node)
	}
	// Push the value (LHS).
	if err := e.withSubexpr(func() error { return e.emitExpr(n.InstanceOfValue()) }); err != nil {
		return err
	}
	// Push the class name (RHS). For static names, OpLoadConst the
	// literal; for dynamic, evaluate the expression — OP_INSTANCEOF
	// accepts either a ZString or a ZObject (it pulls the class name
	// off an object value at runtime).
	if cv := n.InstanceOfClassVar(); cv != nil {
		if err := e.withSubexpr(func() error { return e.emitExpr(cv) }); err != nil {
			return err
		}
	} else {
		idx := e.constIndex(n.InstanceOfStaticClass())
		e.emit(vm.OpLoadConst, idx, 0, 0)
		e.pushStack(1)
	}
	e.emit(vm.OpInstanceOf, 0, 0, 0)
	e.popStack(1) // pops 2, pushes 1 — net -1
	return nil
}

// cloneNode is implemented by *compiler.runnableClone for the basic
// `clone $x` form. The extended PHP 8.5+ forms still AST-delegate.
type cloneNode interface {
	CloneArg() phpv.Runnable
}

// classNameOfNode is implemented by *compiler.runClassNameOf and
// represents `Cls::class` / `$obj::class`.
type classNameOfNode interface {
	ClassNameOfSource() phpv.Runnable
	ClassNameOfIsLiteral() bool
}

// voidCastNode is implemented by *compiler.runVoidCast for `(void) $x`.
type voidCastNode interface {
	VoidCastExpr() phpv.Runnable
}

// firstClassCallableNode is implemented by *compiler.runFirstClassCallable
// for the PHP 8.1 `func(...)` syntax (free-function form only).
type firstClassCallableNode interface {
	FirstClassCallableTarget() phpv.Runnable
}

// methodFirstClassNode is implemented by *compiler.runFirstClassMethodCallable
// for `$obj->m(...)` / `Cls::m(...)` / `$obj?->m(...)` syntax.
type methodFirstClassNode interface {
	MethodFirstClassReceiver() phpv.Runnable
	MethodFirstClassName() phpv.ZString
	MethodFirstClassIsStatic() bool
	MethodFirstClassIsNullsafe() bool
}

// dynMethodFirstClassNode is implemented by
// *compiler.runFirstClassDynMethodCallable for `$obj->{expr}(...)`.
type dynMethodFirstClassNode interface {
	DynMethodFirstClassReceiver() phpv.Runnable
	DynMethodFirstClassNameExpr() phpv.Runnable
}

// objectDynVarReadNode is implemented by *compiler.runObjectDynVar for
// non-nullsafe `$obj->{$name}` reads.
type objectDynVarReadNode interface {
	ObjectDynVarReceiver() phpv.Runnable
	ObjectDynVarNameExpr() phpv.Runnable
}

// classDynConstNode is implemented by *compiler.runClassDynConst for
// `Cls::CONST`, `$obj::CONST`, `Cls::{$name}`.
type classDynConstNode interface {
	ClassDynConstClassExpr() phpv.Runnable
	ClassDynConstNameExpr() phpv.Runnable
}

// classStaticVarReadNode is implemented by *compiler.runClassStaticVarRef
// for `Cls::$prop` reads.
type classStaticVarReadNode interface {
	ClassStaticVarReadClassExpr() phpv.Runnable
	ClassStaticVarReadName() phpv.ZString
}

// classStaticObjRefNode is implemented by *compiler.runClassStaticObjRef
// for `Cls::IDENT` literal-name class const / enum case fetches.
type classStaticObjRefNode interface {
	ClassStaticObjRefClassExpr() phpv.Runnable
	ClassStaticObjRefName() phpv.ZString
}

// classStaticDynVarReadNode is implemented by *compiler.runClassStaticDynVarRef
// for `Cls::${$name}` dyn-name static prop reads.
type classStaticDynVarReadNode interface {
	ClassStaticDynVarReadClassExpr() phpv.Runnable
	ClassStaticDynVarReadNameExpr() phpv.Runnable
}

func (e *emitter) emitClassStaticDynGet(node phpv.Runnable) error {
	n, ok := node.(classStaticDynVarReadNode)
	if !ok {
		return unsupportedf("class-static-dyn-get: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticDynVarReadClassExpr()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticDynVarReadNameExpr()) }); err != nil {
		return err
	}
	e.emit(vm.OpClassStaticDynGet, 0, 0, 0)
	e.popStack(1) // pops 2, pushes 1
	return nil
}

func (e *emitter) emitClassStaticObjRef(node phpv.Runnable) error {
	n, ok := node.(classStaticObjRefNode)
	if !ok {
		return unsupportedf("class-static-objref: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticObjRefClassExpr()) }); err != nil {
		return err
	}
	idx := e.constIndex(n.ClassStaticObjRefName())
	e.emit(vm.OpClassStaticObjRef, idx, 0, 0)
	// net: pops 1, pushes 1 → 0
	return nil
}

func (e *emitter) emitClassStaticGet(node phpv.Runnable) error {
	n, ok := node.(classStaticVarReadNode)
	if !ok {
		return unsupportedf("class-static-get: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticVarReadClassExpr()) }); err != nil {
		return err
	}
	idx := e.constIndex(n.ClassStaticVarReadName())
	e.emit(vm.OpClassStaticGet, idx, 0, 0)
	// net: pops 1, pushes 1 → 0
	return nil
}

// emitStaticPropIncDec emits `Foo::$bar++` / `++Foo::$bar` and dec
// variants for the static-name (*runClassStaticVarRef) shape. The class
// source is evaluated once; OP_INC_DEC_STATIC_PROP dispatches read +
// DoInc + write through EvalClassStaticVarRead / AssignClassStaticProp.
func (e *emitter) emitStaticPropIncDec(target phpv.Runnable, inc bool, post bool, stmtCtx bool) error {
	n, ok := target.(classStaticVarReadNode)
	if !ok {
		return unsupportedf("static-prop inc/dec: node %T doesn't expose accessors", target)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticVarReadClassExpr()) }); err != nil {
		return err
	}
	nameIdx := e.constIndex(n.ClassStaticVarReadName())
	var b uint16
	if inc {
		b |= 1
	}
	if post {
		b |= 2
	}
	var c int32
	if !stmtCtx {
		c = 1
	}
	e.emit(vm.OpIncDecStaticProp, nameIdx, b, c)
	if stmtCtx {
		e.popStack(1) // class-source consumed, nothing pushed
	}
	// expr-ctx: class-source consumed, pre/post pushed → net 0
	return nil
}

// emitStaticPropDynIncDec emits `Cls::${$x}++` / `++Cls::${$x}` and dec
// variants for the dyn-name (*runClassStaticDynVarRef) shape. Both the
// class source and the name expression are evaluated to the stack;
// OP_INC_DEC_STATIC_PROP_DYN reads via EvalClassStaticDynVarRead,
// applies DoInc, and writes via AssignClassStaticDynProp.
func (e *emitter) emitStaticPropDynIncDec(target phpv.Runnable, inc bool, post bool, stmtCtx bool) error {
	n, ok := target.(classStaticDynVarReadNode)
	if !ok {
		return unsupportedf("static-prop dyn inc/dec: node %T doesn't expose accessors", target)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticDynVarReadClassExpr()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassStaticDynVarReadNameExpr()) }); err != nil {
		return err
	}
	var b uint16
	if inc {
		b |= 1
	}
	if post {
		b |= 2
	}
	var c int32
	if !stmtCtx {
		c = 1
	}
	e.emit(vm.OpIncDecStaticPropDyn, 0, b, c)
	if stmtCtx {
		e.popStack(2) // class-source + name consumed, nothing pushed
	} else {
		e.popStack(1) // class+name consumed, pre/post pushed → net -1
	}
	return nil
}

func (e *emitter) emitClassDynConst(node phpv.Runnable) error {
	n, ok := node.(classDynConstNode)
	if !ok {
		return unsupportedf("class-dyn-const: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassDynConstClassExpr()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassDynConstNameExpr()) }); err != nil {
		return err
	}
	e.emit(vm.OpClassDynConst, 0, 0, 0)
	e.popStack(1) // pops 2, pushes 1 → net -1
	return nil
}

func (e *emitter) emitObjectDynGet(node phpv.Runnable) error {
	n, ok := node.(objectDynVarReadNode)
	if !ok {
		return unsupportedf("dyn-obj-get: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ObjectDynVarReceiver()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ObjectDynVarNameExpr()) }); err != nil {
		return err
	}
	var aFlags uint16
	if compiler.ObjectDynVarIsNullSafe(node) {
		aFlags |= 1
	}
	e.emit(vm.OpObjectDynGet, aFlags, 0, 0)
	e.popStack(1) // pops 2, pushes 1 → net -1
	return nil
}

func (e *emitter) emitDynMethodFirstClass(node phpv.Runnable) error {
	n, ok := node.(dynMethodFirstClassNode)
	if !ok {
		return unsupportedf("dyn-method first-class: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.DynMethodFirstClassReceiver()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.DynMethodFirstClassNameExpr()) }); err != nil {
		return err
	}
	e.emit(vm.OpDynMethodFirstClass, 0, 0, 0)
	e.popStack(1) // pops 2, pushes 1 → net -1
	return nil
}

func (e *emitter) emitMethodFirstClass(node phpv.Runnable) error {
	n, ok := node.(methodFirstClassNode)
	if !ok {
		return unsupportedf("method first-class: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.MethodFirstClassReceiver()) }); err != nil {
		return err
	}
	var flags uint16
	if n.MethodFirstClassIsStatic() {
		flags |= 1
	}
	if n.MethodFirstClassIsNullsafe() {
		flags |= 2
	}
	idx := e.constIndex(n.MethodFirstClassName())
	e.emit(vm.OpMethodFirstClass, flags, idx, 0)
	// net stack: 0 (pop 1 recv, push 1 closure)
	return nil
}

func (e *emitter) emitFirstClassCallable(node phpv.Runnable) error {
	n, ok := node.(firstClassCallableNode)
	if !ok {
		return unsupportedf("first-class callable: node %T doesn't expose accessors", node)
	}
	target := n.FirstClassCallableTarget()
	// If the target is a bare constant (function name), push its name
	// as a string literal — the AST's fast path. Otherwise eval normally.
	if cn, ok := target.(constantNode); ok {
		idx := e.constIndex(phpv.ZString(cn.ConstantName()))
		e.emit(vm.OpLoadConst, idx, 0, 0)
		e.pushStack(1)
	} else {
		if err := e.withSubexpr(func() error { return e.emitExpr(target) }); err != nil {
			return err
		}
	}
	e.emit(vm.OpFirstClassCallable, 0, 0, 0)
	// net stack: 0 (pop 1, push 1)
	return nil
}

func (e *emitter) emitVoidCast(node phpv.Runnable) error {
	n, ok := node.(voidCastNode)
	if !ok {
		return unsupportedf("void-cast: node %T doesn't expose accessors", node)
	}
	// Evaluate the inner expression in statement context (discard
	// result) when our outer ctx is also statement context; otherwise
	// evaluate as expression and pop. Always end by pushing NULL when
	// we're producing a value.
	stmtCtx := e.stmtCtx
	if stmtCtx {
		if err := e.emitStmt(n.VoidCastExpr()); err != nil {
			return err
		}
		// statement (void) cast: nothing on the stack, done.
		return nil
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.VoidCastExpr()) }); err != nil {
		return err
	}
	e.emit(vm.OpPop, 0, 0, 0)
	e.popStack(1)
	e.emit(vm.OpLoadNull, 0, 0, 0)
	e.pushStack(1)
	return nil
}

func (e *emitter) emitClassNameOf(node phpv.Runnable) error {
	n, ok := node.(classNameOfNode)
	if !ok {
		return unsupportedf("class-nameof: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ClassNameOfSource()) }); err != nil {
		return err
	}
	var lit uint16
	if n.ClassNameOfIsLiteral() {
		lit = 1
	}
	e.emit(vm.OpClassNameOf, lit, 0, 0)
	// net stack: 0 (pop 1, push 1)
	return nil
}

func (e *emitter) emitClone(node phpv.Runnable) error {
	n, ok := node.(cloneNode)
	if !ok {
		return unsupportedf("clone: node %T doesn't expose accessors", node)
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.CloneArg()) }); err != nil {
		return err
	}
	e.emit(vm.OpClone, 0, 0, 0)
	// net stack: -1 popped + 1 pushed = 0
	return nil
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
	// Reference assignment (`$b = &$a`): AST handles the ref-share
	// semantics. The VM's OpStoreLocal Dups arrays, detaching any
	// inbound ref — wrong for `=&`. For the simple-variable LHS,
	// use OP_STORE_LOCAL_REF which writes the ref ZVal directly.
	// Complex LHS (object prop, array elem, static prop) still
	// AST-delegates — those write paths each have their own ref
	// semantics that we don't yet replicate natively.
	if compiler.IsRefExpr(n.OperatorB()) {
		if v, ok := n.OperatorA().(variableNode); ok {
			stmtCtx := e.stmtCtx
			if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
				return err
			}
			idx := e.localIndex(v.VariableName())
			var b uint16
			if !stmtCtx {
				b |= 1
			}
			e.emit(vm.OpStoreLocalRef, idx, b, 0)
			if stmtCtx {
				e.popStack(1)
			}
			// expr-context: pop+push of same value → net zero.
			return nil
		}
		return e.emitAssignViaAST(n)
	}
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
		// Fast path when the container is a simple local variable —
		// native OP_ARRAY_SET_LOCAL / OP_ARRAY_APPEND_LOCAL.
		if _, ok := aa.ArrayAccessContainer().(variableNode); ok && stmtCtx && !aa.ArrayAccessIsNullSafe() {
			return e.emitArrayAssignToLocal(aa, n.OperatorB(), stmtCtx)
		}
		// Anything else (`$obj->arr[i] = v`, `$a[i][j] = v`,
		// `Foo::$bar[i] = v`, append in expression context, etc.)
		// goes through the AST runner. IsSlotSafe flags these bodies
		// so the hashtable stays authoritative.
		return e.emitAssignViaAST(n)
	}
	// `$obj->prop = v` and `Foo::$bar = v`: typed properties, hooks,
	// asymmetric visibility, indirect-modification notices, and
	// LSB-class resolution all live in runObjectVar.WriteValue /
	// runClassStaticVarRef.WriteValue. Rather than reimplementing
	// those, delegate the whole runOperator (LHS + RHS + write) to
	// the AST runner. IsSlotSafe rejects bodies with these writes,
	// so the hashtable is in sync when the AST reads locals.
	if ov, ok := n.OperatorA().(objectVarNode); ok {
		// `$obj->prop = v` / `$obj->$x = v`: emit natively.
		// emitObjectVarAssign routes the static-name form through
		// OP_OBJECT_SET and the dyn-name ($-prefixed) form through
		// OP_OBJECT_DYN_SET — both dispatch via ZObject.ObjectSet
		// (typed properties, hooks, asymmetric visibility). Expr-context
		// uses the keep-value flag so the assignment result lands on the
		// stack. Nullsafe write (`$obj?->prop = v`) is parse-rejected
		// ("Can't use nullsafe operator in write context") so we never
		// reach here with ov.ObjectVarIsNullSafe() == true.
		return e.emitObjectVarAssign(ov, n.OperatorB(), e.stmtCtx)
	}
	if compiler.IsObjectDynVarReadNode(n.OperatorA()) {
		// `$obj->{$x} = v`: dyn-name property write via OP_OBJECT_DYN_SET.
		// Nullsafe variant is parse-rejected (see objectVarNode branch).
		if dv, ok := n.OperatorA().(objectDynVarReadNode); ok {
			return e.emitObjectDynVarAssign(dv, n.OperatorB(), e.stmtCtx)
		}
		return unsupportedf("object-dyn-var LHS shape: %T", n.OperatorA())
	}
	if compiler.IsStaticPropertyTarget(n.OperatorA()) {
		// `Foo::$bar = v`: emit natively when the LHS is the static-name
		// form (`*runClassStaticVarRef`). AssignClassStaticProp handles
		// LSB resolution, asymmetric visibility, typed-prop enforcement.
		// Expr-context uses the B=1 keep-value flag.
		if compiler.IsClassStaticVarReadNode(n.OperatorA()) {
			classExpr := n.OperatorA().(interface{ ClassStaticVarReadClassExpr() phpv.Runnable }).ClassStaticVarReadClassExpr()
			varName := n.OperatorA().(interface{ ClassStaticVarReadName() phpv.ZString }).ClassStaticVarReadName()
			if err := e.withSubexpr(func() error { return e.emitExpr(classExpr) }); err != nil {
				return err
			}
			if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
				return err
			}
			nameIdx := e.constIndex(varName)
			if e.stmtCtx {
				e.emit(vm.OpStaticPropSet, nameIdx, 0, 0)
				e.popStack(2)
			} else {
				e.emit(vm.OpStaticPropSet, nameIdx, 1, 0)
				e.popStack(1) // pop class+val, push val back → net -1
			}
			return nil
		}
		if compiler.IsClassStaticDynVarReadNode(n.OperatorA()) {
			// `Cls::${$x} = v`: dyn-name static-prop write via
			// OP_STATIC_PROP_DYN_SET. Helper resolves the class,
			// finds the static slot, IncRef/DecRef-tracks the
			// stored object, and writes through SetString.
			n2 := n.OperatorA().(classStaticDynVarReadNode)
			if err := e.withSubexpr(func() error { return e.emitExpr(n2.ClassStaticDynVarReadClassExpr()) }); err != nil {
				return err
			}
			if err := e.withSubexpr(func() error { return e.emitExpr(n2.ClassStaticDynVarReadNameExpr()) }); err != nil {
				return err
			}
			if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
				return err
			}
			var a uint16
			if !e.stmtCtx {
				a |= 1
			}
			e.emit(vm.OpStaticPropDynSet, a, 0, 0)
			if e.stmtCtx {
				e.popStack(3)
			} else {
				e.popStack(2) // pop class+name+val, push val back → net -2
			}
			return nil
		}
		// IsStaticPropertyTarget only returns true for runClassStaticVarRef
		// and runClassStaticDynVarRef — both branches above. Any other
		// shape would be a static-property predicate bug.
		return unsupportedf("static-prop LHS shape: %T", n.OperatorA())
	}
	if compiler.IsDestructureTarget(n.OperatorA()) {
		// `[$a, $b] = $arr` / `list($a, $b) = $arr`: emit the RHS
		// natively, then OP_DESTRUCTURE_ASSIGN runs the shared
		// AssignDestructure helper (which fans out via the AST node's
		// WriteValue — recursive, keyed, ArrayAccess-aware).
		stmtCtx := e.stmtCtx
		if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
			return err
		}
		lhs, ok := n.OperatorA().(phpv.Runnable)
		if !ok {
			return unsupportedf("destructure LHS not a Runnable: %T", n.OperatorA())
		}
		idx := e.astIndex(lhs)
		// In stmt context, drop the value after the assign; in expr
		// context, keep the value (the assignment expression evaluates
		// to the assigned RHS).
		var flags uint16
		if !stmtCtx {
			flags |= 1
		}
		e.emit(vm.OpDestructureAssign, idx, flags, 0)
		if stmtCtx {
			e.popStack(1) // RHS consumed, nothing pushed
		}
		// AST WriteValue may write through ctx.OffsetSet for each target;
		// refresh slots so subsequent slot reads see the new locals.
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	if compiler.IsVariableRef(n.OperatorA()) {
		// `$$name = v` / `${$expr} = v`: emit the name-producing
		// expression and the RHS to the stack; OP_VAR_VAR_SET pops
		// both and routes through compiler.AssignVariableVariable
		// which coerces the name to string and calls ctx.OffsetSet.
		nameExpr := compiler.VarVarNameExpr(n.OperatorA())
		if err := e.withSubexpr(func() error { return e.emitExpr(nameExpr) }); err != nil {
			return err
		}
		if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
			return err
		}
		var a uint16
		if !e.stmtCtx {
			a |= 1
		}
		e.emit(vm.OpVarVarSet, a, 0, 0)
		if e.stmtCtx {
			e.popStack(2)
		} else {
			e.popStack(1) // pop name+val, push val back → net -1
		}
		// The write goes through ctx.OffsetSet; refresh slots so
		// subsequent slot reads see the new value.
		e.emit(vm.OpRefreshSlots, 0, 0, 0)
		return nil
	}
	return unsupportedf("plain `=` to non-variable target %T", n.OperatorA())
}

// emitAssignViaAST delegates a write assignment to the AST runner
// when the LHS shape is one we don't yet lower piecewise (property,
// static property, etc.).
func (e *emitter) emitAssignViaAST(n operatorNode) error {
	raw, ok := n.(phpv.Runnable)
	if !ok {
		return unsupportedf("AST-delegated assign: cannot retrieve raw Runnable")
	}
	stmtCtx := e.stmtCtx
	idx := e.astIndex(raw)
	if stmtCtx {
		e.emit(vm.OpTryFinally, idx, 0, 0) // delegate, discard result
	} else {
		e.emit(vm.OpClassConst, idx, 0, 0) // delegate, push result
		e.pushStack(1)
	}
	e.emit(vm.OpRefreshSlots, 0, 0, 0)
	return nil
}

func (e *emitter) emitCompoundAssign(n operatorNode, op tokenizer.ItemType) error {
	lhs, ok := n.OperatorA().(variableNode)
	if !ok {
		// `$obj->prop OP= rhs`: emit natively for static-name,
		// non-nullsafe property access. OP_OBJECT_COMPOUND_ASSIGN
		// dispatches via objectGet/objectSet (which reach into the
		// AST's ZObject layer for typed props, hooks, etc.) and
		// applies the resolved compoundOp inside the handler.
		if ov, ok := n.OperatorA().(objectVarNode); ok {
			name := ov.ObjectVarName()
			if !ov.ObjectVarIsNullSafe() && !(len(name) > 0 && name[0] == '$') {
				return e.emitObjectVarCompoundAssign(ov, n.OperatorB(), op, e.stmtCtx)
			}
			// `$obj->$x OP= rhs`: dyn-name compound assign. Name is the
			// local variable referenced after the `$` prefix. Receiver
			// write-context suppresses the undef-var warning — pass
			// recvSilent=true to mirror runObjectVar.WriteValue.
			if !ov.ObjectVarIsNullSafe() && len(name) > 0 && name[0] == '$' {
				localIdx := e.localIndex(name[1:])
				pushName := func() error {
					e.emit(vm.OpLoadLocal, localIdx, 0, 0)
					e.pushStack(1)
					return nil
				}
				return e.emitObjectDynVarCompoundAssign(ov.ObjectVarReceiver(), pushName, n.OperatorB(), op, true, e.stmtCtx)
			}
			return e.emitAssignViaAST(n)
		}
		// `$obj->{$x} OP= rhs` (curly-brace dyn-name). runObjectDynVar
		// does NOT suppress the receiver warning — recvSilent=false.
		if compiler.IsObjectDynVarReadNode(n.OperatorA()) {
			if dv, ok := n.OperatorA().(objectDynVarReadNode); ok {
				nameExpr := dv.ObjectDynVarNameExpr()
				pushName := func() error { return e.emitExpr(nameExpr) }
				return e.emitObjectDynVarCompoundAssign(dv.ObjectDynVarReceiver(), pushName, n.OperatorB(), op, false, e.stmtCtx)
			}
			return unsupportedf("object-dyn-var compound LHS shape: %T", n.OperatorA())
		}
		// `$local[k] OP= rhs` on simple-local container: native via
		// OP_ARRAY_COMPOUND_ASSIGN_LOCAL. Reads via arrayGet, snapshots,
		// applies compoundOp, writes back via arraySetLocal. Nested
		// containers (`$obj->arr[i]`, `$a[i][j]`, …) still AST-delegate.
		if aa, ok := n.OperatorA().(arrayAccessNode); ok {
			if v, ok := aa.ArrayAccessContainer().(variableNode); ok &&
				!aa.ArrayAccessIsNullSafe() && aa.ArrayAccessOffset() != nil {
				stmtCtx := e.stmtCtx
				idx := e.localIndex(v.VariableName())
				// Pre-check the container BEFORE evaluating the offset so
				// undefined-variable / null-vivify warnings fire in the
				// right order relative to offset-evaluation warnings, and
				// so string containers reject the compound op before any
				// offset side effects (bug53432, assign_dim_op_undef).
				e.emit(vm.OpArrayPreCheckLocal, idx, 0, 0)
				if err := e.withSubexpr(func() error { return e.emitExpr(aa.ArrayAccessOffset()) }); err != nil {
					return err
				}
				if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
					return err
				}
				var c int32
				if !stmtCtx {
					c = 1
				}
				e.emit(vm.OpArrayCompoundAssignLocal, idx, uint16(op), c)
				if stmtCtx {
					e.popStack(2)
				} else {
					e.popStack(1) // pops offset+rhs, pushes res → net -1
				}
				return nil
			}
			return e.emitAssignViaAST(n)
		}
		if compiler.IsStaticPropertyTarget(n.OperatorA()) {
			// `Foo::$bar OP= rhs`: emit natively when the LHS is the
			// static-name form. The handler dispatches read +
			// compoundOp + write via the existing AssignClassStaticProp
			// + EvalClassStaticVarRead helpers.
			if compiler.IsClassStaticVarReadNode(n.OperatorA()) {
				classExpr := n.OperatorA().(interface{ ClassStaticVarReadClassExpr() phpv.Runnable }).ClassStaticVarReadClassExpr()
				varName := n.OperatorA().(interface{ ClassStaticVarReadName() phpv.ZString }).ClassStaticVarReadName()
				stmtCtx := e.stmtCtx
				if err := e.withSubexpr(func() error { return e.emitExpr(classExpr) }); err != nil {
					return err
				}
				if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
					return err
				}
				nameIdx := e.constIndex(varName)
				var c int32
				if !stmtCtx {
					c = 1
				}
				e.emit(vm.OpStaticPropCompoundAssign, nameIdx, uint16(op), c)
				if stmtCtx {
					e.popStack(2)
				} else {
					e.popStack(1)
				}
				return nil
			}
			if compiler.IsClassStaticDynVarReadNode(n.OperatorA()) {
				// `Cls::${$x} OP= rhs`: dyn-name compound assign.
				// OP_STATIC_PROP_DYN_COMPOUND_ASSIGN pops class+name+rhs,
				// reads via EvalClassStaticDynVarRead, applies compoundOp,
				// writes via AssignClassStaticDynProp.
				n2 := n.OperatorA().(classStaticDynVarReadNode)
				stmtCtx := e.stmtCtx
				if err := e.withSubexpr(func() error { return e.emitExpr(n2.ClassStaticDynVarReadClassExpr()) }); err != nil {
					return err
				}
				if err := e.withSubexpr(func() error { return e.emitExpr(n2.ClassStaticDynVarReadNameExpr()) }); err != nil {
					return err
				}
				if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
					return err
				}
				var c int32
				if !stmtCtx {
					c = 1
				}
				e.emit(vm.OpStaticPropDynCompoundAssign, 0, uint16(op), c)
				if stmtCtx {
					e.popStack(3)
				} else {
					e.popStack(2) // pop class+name+rhs, push result → net -2
				}
				return nil
			}
			// IsStaticPropertyTarget only returns true for the two
			// shapes above; any other would be a predicate bug.
			return unsupportedf("static-prop compound LHS shape: %T", n.OperatorA())
		}
		return unsupportedf("compound assign to non-variable target %T", n.OperatorA())
	}
	stmtCtx := e.stmtCtx
	if err := e.withSubexpr(func() error { return e.emitExpr(n.OperatorB()) }); err != nil {
		return err
	}
	idx := e.localIndex(lhs.VariableName())
	e.emit(vm.OpOpAssignLocal, idx, uint16(op), 0)
	e.popStack(1)
	// Expression context (`$y = ($x += 1) * 2`): leave the post-
	// modification value on the stack.
	if !stmtCtx {
		e.emit(vm.OpLoadLocal, idx, 0, 0)
		e.pushStack(1)
	}
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
		// `$obj->prop++` / `++$obj->prop` / `--`: emit natively for
		// static-name, non-nullsafe property access.
		if ov, ok := target.(objectVarNode); ok {
			name := ov.ObjectVarName()
			if !ov.ObjectVarIsNullSafe() && !(len(name) > 0 && name[0] == '$') {
				return e.emitObjectVarIncDec(ov, inc, post, e.stmtCtx)
			}
			// `$obj->$x++` dyn-name inc/dec. Mirrors compound-assign
			// dollar-prefix branch: silent receiver, name comes from
			// local var after the `$` prefix.
			if !ov.ObjectVarIsNullSafe() && len(name) > 0 && name[0] == '$' {
				localIdx := e.localIndex(name[1:])
				pushName := func() error {
					e.emit(vm.OpLoadLocal, localIdx, 0, 0)
					e.pushStack(1)
					return nil
				}
				return e.emitObjectDynVarIncDec(ov.ObjectVarReceiver(), pushName, inc, post, true, e.stmtCtx)
			}
			return e.emitAssignViaAST(n)
		}
		// `$obj->{$x}++` curly-brace dyn-name inc/dec.
		if compiler.IsObjectDynVarReadNode(target) {
			if dv, ok := target.(objectDynVarReadNode); ok {
				nameExpr := dv.ObjectDynVarNameExpr()
				pushName := func() error { return e.emitExpr(nameExpr) }
				return e.emitObjectDynVarIncDec(dv.ObjectDynVarReceiver(), pushName, inc, post, false, e.stmtCtx)
			}
			return unsupportedf("object-dyn-var inc/dec shape: %T", target)
		}
		// `Foo::$bar++` / `++Foo::$bar` / `--`: emit natively for the
		// static-name form (*runClassStaticVarRef).
		if compiler.IsStaticPropertyTarget(target) && compiler.IsClassStaticVarReadNode(target) {
			return e.emitStaticPropIncDec(target, inc, post, e.stmtCtx)
		}
		// `Cls::${$x}++` / `++Cls::${$x}` / dec dyn-name static-prop.
		if compiler.IsClassStaticDynVarReadNode(target) {
			return e.emitStaticPropDynIncDec(target, inc, post, e.stmtCtx)
		}
		// `$local[k]++` / `++$local[k]` / dec on a simple-local
		// container: native via OP_ARRAY_INC_DEC_LOCAL. Nested
		// containers (`$obj->arr[i]`, `$a[i][j]`, …) still AST-delegate.
		if aa, ok := target.(arrayAccessNode); ok {
			if v, ok := aa.ArrayAccessContainer().(variableNode); ok &&
				!aa.ArrayAccessIsNullSafe() && aa.ArrayAccessOffset() != nil {
				stmtCtx := e.stmtCtx
				idx := e.localIndex(v.VariableName())
				// Same pre-check as compound assign (see comment there).
				e.emit(vm.OpArrayPreCheckLocal, idx, 0, 0)
				if err := e.withSubexpr(func() error { return e.emitExpr(aa.ArrayAccessOffset()) }); err != nil {
					return err
				}
				var b uint16
				if inc {
					b |= 1
				}
				if post {
					b |= 2
				}
				var c int32
				if !stmtCtx {
					c = 1
				}
				e.emit(vm.OpArrayIncDecLocal, idx, b, c)
				if stmtCtx {
					e.popStack(1)
				}
				// expr-context: pops offset, pushes res → net 0.
				return nil
			}
		}
		// Other cases (`$obj->arr[i]++`, `$a[i][j]++`, …) still
		// AST-delegate.
		return e.emitAssignViaAST(n)
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

// isCoalesceableArrayChain reports whether r is a non-nullsafe
// array-access chain whose container is a simple variable (possibly
// through nested array-access). Matches the shapes
// emitIssetContainerRead handles.
func isCoalesceableArrayChain(r phpv.Runnable) bool {
	if !compiler.IsArrayAccessNode(r) {
		return false
	}
	cont, _ := compiler.ArrayAccessParts(r)
	if compiler.IsSimpleVariable(cont) {
		return true
	}
	return isCoalesceableArrayChain(cont)
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
	} else if compiler.IsArrayAccessNode(a) && isCoalesceableArrayChain(a) {
		// `$container[$key1][$key2]? ?? default` — use the same
		// permissive chain reads as nested isset (no warnings on
		// missing intermediate / final keys, no string-on-string
		// TypeError). The chain leaves the value or null on top.
		if err := e.emitIssetContainerRead(a); err != nil {
			return err
		}
	} else if ov, ok := a.(objectVarNode); ok && len(ov.ObjectVarName()) > 0 && ov.ObjectVarName()[0] != '$' {
		// `$obj->prop ?? default` — emit OP_OBJECT_GET_SAFE which
		// silences "Undefined property" warnings on object receivers
		// (matches PHP's `??` semantics). Skip the dollar-prefix
		// (runtime var-var) form — falls through to AST delegation.
		if err := e.withSubexpr(func() error { return e.emitExpr(ov.ObjectVarReceiver()) }); err != nil {
			return err
		}
		idx := e.constIndex(ov.ObjectVarName())
		var bFlag uint16
		if ov.ObjectVarIsNullSafe() {
			bFlag = 1
		}
		e.emit(vm.OpObjectGetSafe, idx, bFlag, 0)
		// pop receiver, push value → net stack delta zero.
	} else {
		// LHS shapes that need write-context suppression (object
		// props, nullsafe chains) — delegate the entire `LHS ?? RHS`
		// expression to the AST runner.
		raw, ok := n.(phpv.Runnable)
		if !ok {
			return unsupportedf("coalesce AST delegation: cannot retrieve raw Runnable")
		}
		idx := e.astIndex(raw)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
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

// emitIssetEmptySupported emits the natively-handled form of
// isset(…) / empty(…). For empty() the single arg's shape determines
// the opcode (LOCAL or DIM). For isset() each arg emits its own
// bool, joined by short-circuit jumps so the first false short-
// circuits to the end with `false` on the stack.
func (e *emitter) emitIssetEmptySupported(node phpv.Runnable) error {
	if arg := compiler.EmptyArg(node); arg != nil {
		return e.emitEmptyArg(arg)
	}
	args := compiler.IssetArgs(node)
	if len(args) == 0 {
		e.emit(vm.OpLoadTrue, 0, 0, 0)
		e.pushStack(1)
		return nil
	}
	jumps := make([]uint32, 0, len(args)-1)
	for i, a := range args {
		if err := e.emitIssetArg(a); err != nil {
			return err
		}
		if i == len(args)-1 {
			break
		}
		jumps = append(jumps, e.emit(vm.OpJmpIfFalsePeek, 0, 0, 0))
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	end := uint32(len(e.code))
	for _, j := range jumps {
		e.patchJump(j, end)
	}
	return nil
}

// emitIssetArg emits a single isset argument; pushes one bool.
func (e *emitter) emitIssetArg(a phpv.Runnable) error {
	if compiler.IsSimpleVariable(a) {
		idx := e.localIndex(compiler.SimpleVariableName(a))
		e.emit(vm.OpIssetLocal, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsArrayAccessNode(a) {
		cont, off := compiler.ArrayAccessParts(a)
		if err := e.withSubexpr(func() error { return e.emitIssetContainerRead(cont) }); err != nil {
			return err
		}
		if err := e.withSubexpr(func() error { return e.emitExpr(off) }); err != nil {
			return err
		}
		e.emit(vm.OpIssetDim, 0, 0, 0)
		e.popStack(1) // 2 in, 1 out
		return nil
	}
	if ov, ok := a.(objectVarNode); ok {
		// `isset($obj->prop)` — emit receiver as a permissive read so
		// undefined-variable warnings on the receiver are suppressed,
		// then OP_ISSET_OBJ_PROP pops receiver + reads name from the
		// const pool to dispatch to EvalIssetObjProp.
		if err := e.withSubexpr(func() error { return e.emitIssetContainerRead(ov.ObjectVarReceiver()) }); err != nil {
			return err
		}
		idx := e.constIndex(ov.ObjectVarName())
		e.emit(vm.OpIssetObjProp, idx, 0, 0)
		// pop receiver, push bool → net stack delta zero.
		return nil
	}
	if compiler.IsClassStaticVarReadNode(a) {
		// `isset(Foo::$bar)` — emit class-source then OP_ISSET_STATIC_PROP.
		classExpr := a.(interface{ ClassStaticVarReadClassExpr() phpv.Runnable }).ClassStaticVarReadClassExpr()
		varName := a.(interface{ ClassStaticVarReadName() phpv.ZString }).ClassStaticVarReadName()
		if err := e.withSubexpr(func() error { return e.emitExpr(classExpr) }); err != nil {
			return err
		}
		idx := e.constIndex(varName)
		e.emit(vm.OpIssetStaticProp, idx, 0, 0)
		// pop class-source, push bool → net stack delta zero.
		return nil
	}
	return unsupportedf("emitIssetArg: unsupported shape %T", a)
}

// emitEmptyArg emits the single empty(…) argument; pushes one bool.
func (e *emitter) emitEmptyArg(a phpv.Runnable) error {
	if compiler.IsSimpleVariable(a) {
		idx := e.localIndex(compiler.SimpleVariableName(a))
		e.emit(vm.OpEmptyLocal, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsArrayAccessNode(a) {
		cont, off := compiler.ArrayAccessParts(a)
		if err := e.withSubexpr(func() error { return e.emitIssetContainerRead(cont) }); err != nil {
			return err
		}
		if err := e.withSubexpr(func() error { return e.emitExpr(off) }); err != nil {
			return err
		}
		e.emit(vm.OpEmptyDim, 0, 0, 0)
		e.popStack(1)
		return nil
	}
	if ov, ok := a.(objectVarNode); ok {
		// `empty($obj->prop)` — emit receiver permissively then
		// OP_EMPTY_OBJ_PROP. The dispatch returns true for non-object
		// receivers and missing properties (matches PHP semantics).
		if err := e.withSubexpr(func() error { return e.emitIssetContainerRead(ov.ObjectVarReceiver()) }); err != nil {
			return err
		}
		idx := e.constIndex(ov.ObjectVarName())
		e.emit(vm.OpEmptyObjProp, idx, 0, 0)
		// pop receiver, push bool → net stack delta zero.
		return nil
	}
	if compiler.IsClassStaticVarReadNode(a) {
		// `empty(Foo::$bar)` — emit class-source then
		// OP_EMPTY_STATIC_PROP. Missing-class or undeclared-prop
		// returns true (empty) without warning, matching PHP's
		// "any error → empty" behaviour from checkEmpty's default.
		classExpr := a.(interface{ ClassStaticVarReadClassExpr() phpv.Runnable }).ClassStaticVarReadClassExpr()
		varName := a.(interface{ ClassStaticVarReadName() phpv.ZString }).ClassStaticVarReadName()
		if err := e.withSubexpr(func() error { return e.emitExpr(classExpr) }); err != nil {
			return err
		}
		idx := e.constIndex(varName)
		e.emit(vm.OpEmptyStaticProp, idx, 0, 0)
		// pop class-source, push bool → net stack delta zero.
		return nil
	}
	return unsupportedf("emitEmptyArg: unsupported shape %T", a)
}

// emitIssetContainerRead emits a read of an isset/empty container.
// For a simple variable, uses OP_LOAD_LOCAL (no "Undefined variable"
// warning — isset reads should not warn on missing names). For a
// nested array-access, recurses.
func (e *emitter) emitIssetContainerRead(c phpv.Runnable) error {
	if compiler.IsSimpleVariable(c) {
		idx := e.localIndex(compiler.SimpleVariableName(c))
		e.emit(vm.OpLoadLocal, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsArrayAccessNode(c) {
		// Nested: $a[$k1][$k2]. Recurse on the inner container, then
		// fetch via OP_ARRAY_GET_SAFE — isset's permissive semantics
		// silently return null on missing intermediate keys or
		// TypeError-shaped accesses (e.g. string-on-string offsets).
		cont, off := compiler.ArrayAccessParts(c)
		if err := e.emitIssetContainerRead(cont); err != nil {
			return err
		}
		if err := e.withSubexpr(func() error { return e.emitExpr(off) }); err != nil {
			return err
		}
		e.emit(vm.OpArrayGetSafe, 0, 0, 0)
		e.popStack(1)
		return nil
	}
	if ov, ok := c.(objectVarNode); ok {
		// Nested `$outer->inner->...` — recurse for the receiver, then
		// permissive object read so a missing intermediate property
		// just produces null without warning.
		name := ov.ObjectVarName()
		if !ov.ObjectVarIsNullSafe() && len(name) > 0 && name[0] != '$' {
			if err := e.emitIssetContainerRead(ov.ObjectVarReceiver()); err != nil {
				return err
			}
			idx := e.constIndex(name)
			e.emit(vm.OpObjectGetSafe, idx, 0, 0)
			return nil
		}
		// Fall through to unsupported for nullsafe/dyn-name nested
		// containers — those keep AST delegation.
	}
	return unsupportedf("emitIssetContainerRead: unsupported shape %T", c)
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

// emitMatch lowers `match (…) { … }` as a value-producing expression.
//
// Layout:
//
//	push cond                # [cond]
//	for each arm:
//	  for each case:
//	    DUP                  # [cond, cond]
//	    push case            # [cond, cond, c]
//	    CMP_ID               # [cond, bool]
//	    JMP_IF_TRUE arm_body # consumes bool → [cond]
//	# no arm matched:
//	if default:
//	  POP cond               # []
//	  emit default body      # [result]
//	  JMP end
//	else:
//	  OP_THROW_UNHANDLED_MATCH # pops cond, throws
//	# (each arm body)
//	arm_body_i:
//	  POP cond               # []
//	  emit body              # [result]
//	  JMP end
//	end:
//
// Branches are mutually exclusive at runtime — each leaves a single
// value on the stack.
func (e *emitter) emitMatch(n compiler.MatchNode) error {
	if err := e.withSubexpr(func() error { return e.emitExpr(n.MatchCond()) }); err != nil {
		return err
	}
	// cond sits on top of stack throughout the dispatch chain.

	arms := n.MatchArms()
	armBodyJumps := make([]uint32, len(arms))
	for i := range armBodyJumps {
		armBodyJumps[i] = 0xFFFFFFFF // patched as we discover the body PC
	}
	armPatchTargets := make([][]uint32, len(arms))

	for i, arm := range arms {
		for _, c := range arm.MatchArmConditions() {
			e.emit(vm.OpDup, 0, 0, 0)
			e.pushStack(1)
			if err := e.withSubexpr(func() error { return e.emitExpr(c) }); err != nil {
				return err
			}
			e.emit(vm.OpCmpId, 0, 0, 0)
			e.popStack(1) // pops 2 (cond_dup + c), pushes 1 (bool)
			j := e.emit(vm.OpJmpIfTrue, 0, 0, 0)
			e.popStack(1) // consumes bool
			armPatchTargets[i] = append(armPatchTargets[i], j)
		}
	}

	// All arms exhausted; stack still has [cond].
	endJumps := []uint32{}
	if def := n.MatchDefaultBody(); def != nil {
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
		if err := e.withSubexpr(func() error { return e.emitExpr(def) }); err != nil {
			return err
		}
		endJumps = append(endJumps, e.emit(vm.OpJmp, 0, 0, 0))
		e.popStack(1) // remove the default-body result from the simulated stack so arm bodies start clean
	} else {
		e.emit(vm.OpThrowUnhandledMatch, 0, 0, 0)
		e.popStack(1) // throw consumes cond
	}

	// Each arm body: jump targets patched to here.
	for i, arm := range arms {
		armPC := uint32(len(e.code))
		for _, p := range armPatchTargets[i] {
			e.patchJump(p, armPC)
		}
		// Stack at entry: [cond].
		e.pushStack(1) // restore simulated stack for arm body entry
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
		if err := e.withSubexpr(func() error { return e.emitExpr(arm.MatchArmBody()) }); err != nil {
			return err
		}
		endJumps = append(endJumps, e.emit(vm.OpJmp, 0, 0, 0))
		e.popStack(1) // remove arm result so next iteration starts clean
	}

	end := uint32(len(e.code))
	for _, j := range endJumps {
		e.patchJump(j, end)
	}
	// One value lands on the stack at runtime, regardless of branch.
	e.pushStack(1)
	return nil
}

// emitTernary lowers `cond ? a : b` (and `cond ?: alt`) as a value-
// producing expression. Each branch leaves exactly one value on the
// stack; the simulated stack tracker counts that as a single net
// push (branches are mutually exclusive at runtime).
func (e *emitter) emitTernary(n ifNode) error {
	if n.IfIsShortTernary() {
		// `cond ?: alt` — evaluate cond once; if truthy, leave it
		// on the stack; otherwise drop it and evaluate alt.
		if err := e.withSubexpr(func() error { return e.emitExpr(n.IfCond()) }); err != nil {
			return err
		}
		// JMP_IF_FALSE_PEEK keeps the value on the stack for the
		// true side, pops it on the false side.
		jmpFalse := e.emit(vm.OpJmpIfFalsePeek, 0, 0, 0)
		// True branch: value is already on the stack — jump to end.
		jmpEnd := e.emit(vm.OpJmp, 0, 0, 0)
		// False branch: PEEK popped the value, eval alt to push 1.
		e.patchJump(jmpFalse, uint32(len(e.code)))
		e.popStack(1)
		if err := e.withSubexpr(func() error { return e.emitExpr(n.IfNo()) }); err != nil {
			return err
		}
		e.patchJump(jmpEnd, uint32(len(e.code)))
		return nil
	}
	// Full ternary.
	if err := e.withSubexpr(func() error { return e.emitExpr(n.IfCond()) }); err != nil {
		return err
	}
	jmpFalse := e.emit(vm.OpJmpIfFalse, 0, 0, 0)
	e.popStack(1)
	// True branch.
	if err := e.withSubexpr(func() error { return e.emitExpr(n.IfYes()) }); err != nil {
		return err
	}
	jmpEnd := e.emit(vm.OpJmp, 0, 0, 0)
	// False branch.
	e.patchJump(jmpFalse, uint32(len(e.code)))
	e.popStack(1) // simulator: discount the true branch's push so the false branch's emitExpr starts at the same depth
	if err := e.withSubexpr(func() error { return e.emitExpr(n.IfNo()) }); err != nil {
		return err
	}
	e.patchJump(jmpEnd, uint32(len(e.code)))
	return nil
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

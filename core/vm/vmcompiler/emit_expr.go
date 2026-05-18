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
	if compiler.IsClassConstNode(node) {
		// Foo::CONST, Foo::$bar, Foo::class, self::… — delegate to
		// the AST runner.
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsIssetOrEmptyNode(node) {
		// All-simple-local isset/empty lowers natively. Complex forms
		// (array access, object prop, dyn names, …) keep the AST path.
		if compiler.IssetEmptyAllSimple(node) {
			return e.emitIssetEmptySimple(node)
		}
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsAnonymousClassNode(node) {
		// `new class { … }` — AST runs the class registration +
		// instantiation; push the resulting object.
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
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
	if compiler.IsValueExprAstDelegated(node) {
		// Common value-expression types we delegate wholesale
		// (clone, instanceof, void-cast, first-class callables, …).
		idx := e.astIndex(node)
		e.emit(vm.OpClassConst, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	if compiler.IsUnsetNode(node) {
		// All-simple-local unset lowers natively; complex forms keep
		// the AST path. unset() returns nothing, so non-statement use
		// of the AST form needs a trailing null push.
		stmtCtx := e.stmtCtx
		if compiler.UnsetAllSimple(node) {
			for _, a := range compiler.UnsetArgs(node) {
				idx := e.localIndex(compiler.SimpleVariableName(a))
				e.emit(vm.OpUnsetLocal, idx, 0, 0)
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
	cName, noFallback, _ := compiler.ConstantNameAndFlags(raw)
	idx := e.constIndex(phpv.ZString(cName))
	var bFlags uint16
	if noFallback {
		bFlags |= 1
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
	e.emit(vm.OpObjectDynGet, 0, 0, 0)
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
	// inbound ref — wrong for `=&`.
	if compiler.IsRefExpr(n.OperatorB()) {
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
	if _, ok := n.OperatorA().(objectVarNode); ok {
		return e.emitAssignViaAST(n)
	}
	if compiler.IsStaticPropertyTarget(n.OperatorA()) {
		return e.emitAssignViaAST(n)
	}
	if compiler.IsDestructureTarget(n.OperatorA()) {
		// `[$a, $b] = $arr` / `list($a, $b) = $arr`: AST handles
		// the recursive destructure including nested lists and
		// keyed entries.
		return e.emitAssignViaAST(n)
	}
	if compiler.IsVariableRef(n.OperatorA()) {
		// `$$name = …` — AST handles the dynamic name resolution.
		return e.emitAssignViaAST(n)
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
		// Delegate property / static-prop / array-element compound
		// writes to the AST.
		switch n.OperatorA().(type) {
		case objectVarNode, arrayAccessNode:
			return e.emitAssignViaAST(n)
		}
		if compiler.IsStaticPropertyTarget(n.OperatorA()) {
			return e.emitAssignViaAST(n)
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
		// ++/-- on $obj->prop, $arr[$k], Foo::$bar, etc. — route
		// through the AST. The whole runOperator is a write op.
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
		// LHS shapes that need write-context suppression (array
		// elements, object props, nullsafe chains) — delegate the
		// entire `LHS ?? RHS` expression to the AST runner.
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

// emitIssetEmptySimple emits the all-simple-local form of isset(…)
// or empty(…). For empty(), there's exactly one arg → one
// OP_EMPTY_LOCAL. For isset(), each arg becomes an OP_ISSET_LOCAL
// joined by short-circuit jumps: the first false short-circuits to
// the end with `false` on the stack.
func (e *emitter) emitIssetEmptySimple(node phpv.Runnable) error {
	if arg := compiler.EmptyArg(node); arg != nil {
		idx := e.localIndex(compiler.SimpleVariableName(arg))
		e.emit(vm.OpEmptyLocal, idx, 0, 0)
		e.pushStack(1)
		return nil
	}
	args := compiler.IssetArgs(node)
	// `isset()` with no args is a parse error in PHP — defensive: push true.
	if len(args) == 0 {
		e.emit(vm.OpLoadTrue, 0, 0, 0)
		e.pushStack(1)
		return nil
	}
	// Emit each arg's OP_ISSET_LOCAL; short-circuit on first false.
	jumps := make([]uint32, 0, len(args)-1)
	for i, a := range args {
		idx := e.localIndex(compiler.SimpleVariableName(a))
		e.emit(vm.OpIssetLocal, idx, 0, 0)
		e.pushStack(1)
		if i == len(args)-1 {
			break
		}
		// Peek-jump: keep false on stack and jump to end.
		jumps = append(jumps, e.emit(vm.OpJmpIfFalsePeek, 0, 0, 0))
		// Otherwise drop the true and continue with next arg.
		e.emit(vm.OpPop, 0, 0, 0)
		e.popStack(1)
	}
	end := uint32(len(e.code))
	for _, j := range jumps {
		e.patchJump(j, end)
	}
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

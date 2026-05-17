package compiler

import (
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// TryBuildVMClosureBody is an optional hook installed by the
// core/vm/vmcompiler package at init time. When set, the compiler
// invokes it after building each closure body (`function foo() { ... }`,
// closures, arrow fns) and, if it returns a non-nil Runnable, replaces
// the AST body with the VM-backed runner.
//
// hasByRef tells the VM emitter whether any parameter (or `use`
// capture) is by-reference. The VM passes values, but the AST
// callBody binds by-ref args into the FuncContext hashtable before
// the VM body runs; when hasByRef is true the body must avoid
// SlotOnly so writes propagate through the ref via OffsetSet.
//
// Returning nil means "VM compilation failed or was declined; keep the
// AST body". Implementations must be safe to call concurrently and
// must not mutate the input Runnable.
//
// Set to nil (the zero value) when the vmcompiler is not linked or has
// disabled itself via env var. Compiler treats nil as "VM disabled".
var TryBuildVMClosureBody func(ctx phpv.Context, name phpv.ZString, src *phpv.Loc, body phpv.Runnable, hasByRef bool) phpv.Runnable

// TryBuildVMScript is the equivalent hook for top-level scripts. It
// gets the full Runnables and may return a wrapping VM runner.
var TryBuildVMScript func(ctx phpv.Context, src *phpv.Loc, body phpv.Runnable) phpv.Runnable

// unwrapVMBody peels off any VM-runtime wrapping installed by the
// vmcompiler so introspection paths (Dump, validate-break, yield
// collection) keep operating on the original AST. The wrapper exposes
// Inner() returning the AST it was built from. Returns r unchanged
// when r is not a VM wrapper.
func unwrapVMBody(r phpv.Runnable) phpv.Runnable {
	if r == nil {
		return nil
	}
	if inner, ok := r.(interface{ Inner() phpv.Runnable }); ok {
		return inner.Inner()
	}
	return r
}

// IsVMCompiled reports whether a callable's body has been replaced by
// a VM-backed runner. Returns false for AST-only callables, anything
// non-ZClosure, or when the callable isn't a function (e.g. internal
// builtins). Used by tests and tooling to confirm the VM gate took
// effect for a given function.
func IsVMCompiled(c phpv.Callable) bool {
	zc, ok := c.(*ZClosure)
	if !ok || zc == nil || zc.code == nil {
		return false
	}
	if _, ok := zc.code.(interface {
		Inner() phpv.Runnable
	}); ok {
		// ClosureBody and any future wrapper expose Inner().
		return true
	}
	return false
}

// This file exposes a small set of accessor methods on the unexported
// AST node types so that out-of-package consumers (notably
// core/vm/vmcompiler) can pattern-match against them and read the
// fields they need to emit bytecode.
//
// The accessors are read-only and intentionally narrow — they expose
// what the bytecode emitter needs to translate an AST node, not
// internal compiler state. Each method is a one-line getter so it
// adds zero runtime cost when unused (Go inlines + the AST executor
// continues to read fields directly).

// --- runNoDiscardStatement --------------------------------------------

// NoDiscardInner returns the wrapped statement. The VM emitter
// unwraps this, foregoing the NoDiscard warning for VM-compiled
// statements — a small loss until we wire up the warning natively.
func (r *runNoDiscardStatement) NoDiscardInner() phpv.Runnable { return r.inner }

// --- runClassDynConst / runClassStaticObjRef / runClassStaticVarRef --

// IsClassConstNode reports whether r is one of the AST nodes for
// class-level constant / static-property / static-method-name access:
// `Foo::CONST`, `Foo::{$expr}`, `Foo::$bar`, `self::method`, etc.
// Visibility, interface/parent walking, CompileDelayed resolution and
// late-static-binding all live in the AST runners; the VM delegates
// via OP_CLASS_CONST.
func IsClassConstNode(r phpv.Runnable) bool {
	switch r.(type) {
	case *runClassDynConst:
		return true
	case *runClassStaticObjRef:
		return true
	case *runClassStaticVarRef:
		return true
	case *runClassStaticDynVarRef:
		return true
	}
	return false
}

// IsStaticPropertyTarget reports whether r is a static-property
// reference (`Foo::$bar` or `Foo::{$expr}`) — i.e. a valid LHS for an
// assignment that we delegate to the AST.
func IsStaticPropertyTarget(r phpv.Runnable) bool {
	switch r.(type) {
	case *runClassStaticVarRef:
		return true
	case *runClassStaticDynVarRef:
		return true
	}
	return false
}

// --- ZClosure as expression -------------------------------------------

// IsClosureNode reports whether r is a *ZClosure (an inline closure
// or named-function-declaration AST node). When the VM emitter sees
// this it emits an OpMakeClosure that runs ZClosure.Run — that handles
// both registration of named functions AND `dup + capture + Spawn`
// for anonymous closures.
func IsClosureNode(r phpv.Runnable) bool {
	_, ok := r.(*ZClosure)
	return ok
}

// IsUnsetNode reports whether r is an `unset(…)` statement. The VM
// emitter routes these through OpTryFinally (void) + OP_REFRESH_SLOTS.
func IsUnsetNode(r phpv.Runnable) bool {
	_, ok := r.(*runnableUnset)
	return ok
}

// IsAnonymousClassNode reports whether r is a `new class { … }`
// instantiation. The VM AST-delegates these wholesale via OpClassConst.
func IsAnonymousClassNode(r phpv.Runnable) bool {
	_, ok := r.(*runNewAnonymousClass)
	return ok
}

// IsMatchNode reports whether r is a `match (…) { … }` expression.
// The VM AST-delegates these via OpClassConst (push result).
func IsMatchNode(r phpv.Runnable) bool {
	_, ok := r.(*runMatch)
	return ok
}

// IsSwitchNode reports whether r is a `switch (…) { … }` statement.
// The VM AST-delegates via OpTryFinally (void) + OP_REFRESH_SLOTS.
func IsSwitchNode(r phpv.Runnable) bool {
	_, ok := r.(*runSwitch)
	return ok
}

// IsGlobalOrStaticDecl reports whether r is a `global $x;` or
// `static $y = 1;` declaration. The VM AST-delegates these so the
// FuncContext bindings happen via the AST; OP_REFRESH_SLOTS afterwards
// makes the declared locals visible in the slot cache.
func IsGlobalOrStaticDecl(r phpv.Runnable) bool {
	switch r.(type) {
	case *runGlobal, *runStaticVar:
		return true
	}
	return false
}

// IsDestructureTarget reports whether r is a `list(…)` or `[…]`
// destructuring LHS. The VM AST-delegates these writes.
func IsDestructureTarget(r phpv.Runnable) bool {
	_, ok := r.(*runDestructure)
	return ok
}

// IsVariableRef reports whether r is a `$$name` / `${$expr}` variable-
// variable reference. The VM AST-delegates these reads/writes.
func IsVariableRef(r phpv.Runnable) bool {
	_, ok := r.(*runVariableRef)
	return ok
}

// IsValueExprAstDelegated reports whether r is a value-producing
// expression the VM AST-delegates wholesale via OpClassConst. The
// AST runner reads its operands from the FuncContext hashtable.
// IsSlotSafe rejects bodies containing any of these so the hashtable
// stays authoritative.
func IsValueExprAstDelegated(r phpv.Runnable) bool {
	switch n := r.(type) {
	case *runnableClone:
		// Basic `clone $x` is now native; only the extended PHP 8.5+
		// forms (clone with withProperties, clone(...$arr), named args)
		// still AST-delegate.
		return !n.CloneIsBasic()
	}
	switch r.(type) {
	case *runFirstClassCallable,
		*runFirstClassCloneCallable,
		*runFirstClassMethodCallable,
		*runFirstClassDynMethodCallable,
		*runObjectDynVar,
		*runObjectDynFunc,
		*runRef:
		return true
	}
	return false
}

// IsInstanceOfNode reports whether r is a `$v instanceof Cls` expression.
// The VM lowers it to OpInstanceOf.
func IsInstanceOfNode(r phpv.Runnable) bool {
	_, ok := r.(*runInstanceOf)
	return ok
}

// IsBasicCloneNode reports whether r is a basic `clone $x` (no
// withProperties, no spread, no named args). The VM lowers these
// to OpClone. The extended PHP 8.5+ forms keep AST-delegating.
func IsBasicCloneNode(r phpv.Runnable) bool {
	c, ok := r.(*runnableClone)
	return ok && c.CloneIsBasic()
}

// IsClassNameOfNode reports whether r is a `Cls::class` / `$obj::class`
// expression. The VM lowers it to OpClassNameOf.
func IsClassNameOfNode(r phpv.Runnable) bool {
	_, ok := r.(*runClassNameOf)
	return ok
}

// ClassNameOfSource returns the class-source expression (a class name
// literal, `self`/`parent`/`static`, or an object expression). Caller
// also needs ClassNameOfIsLiteral to disambiguate error messages.
func (r *runClassNameOf) ClassNameOfSource() phpv.Runnable { return r.className }

// ClassNameOfIsLiteral reports whether the class-source expression was
// a compile-time literal — affects which error message is used when
// the runtime value isn't a string/object.
func (r *runClassNameOf) ClassNameOfIsLiteral() bool {
	switch cn := r.className.(type) {
	case *runZVal:
		_ = cn
		return true
	case *runParentheses:
		if _, ok := cn.r.(*runZVal); ok {
			return true
		}
	}
	return false
}

// ClassNameOfLoc returns the source loc of the `::class` operator.
func (r *runClassNameOf) ClassNameOfLoc() *phpv.Loc { return r.l }

// IsVoidCastNode reports whether r is a `(void) $expr` cast. The VM
// lowers it natively: eval the expression, discard the value, push
// NULL.
func IsVoidCastNode(r phpv.Runnable) bool {
	_, ok := r.(*runVoidCast)
	return ok
}

// VoidCastExpr returns the inner expression of `(void) expr`.
func (r *runVoidCast) VoidCastExpr() phpv.Runnable { return r.expr }

// IsInlineHtmlNode reports whether r is a literal inline-HTML chunk
// (the text between `?>` and `<?php`). The VM lowers these to
// OP_INLINE_HTML which writes the bytes directly.
func IsInlineHtmlNode(r phpv.Runnable) bool {
	_, ok := r.(runInlineHtml)
	return ok
}

// InlineHtmlText returns the literal text of an inline-HTML chunk.
func InlineHtmlText(r phpv.Runnable) (phpv.ZString, bool) {
	if s, ok := r.(runInlineHtml); ok {
		return phpv.ZString(s), true
	}
	return "", false
}

// IsRefExpr reports whether r is a `&$expr` reference-producing wrapper.
// The VM emitter uses this to detect reference assignment (`$b = &$a`)
// so the whole assignment AST-delegates: storeLocal would otherwise
// `Nude().Dup()` the array, detaching the ref the caller expects.
func IsRefExpr(r phpv.Runnable) bool {
	_, ok := r.(*runRef)
	return ok
}

// NewObjectMethodIterator returns a ZIterator that drives an
// object implementing the Iterator interface via its rewind/valid/
// current/key/next methods. This is what the AST's runnableForeach
// uses for objects (including Generators); the VM mirrors it.
//
// Returns nil if obj is nil. Caller is responsible for calling
// Reset() before iteration.
func NewObjectMethodIterator(ctx phpv.Context, obj any) phpv.ZIterator {
	zo, ok := obj.(*phpobj.ZObject)
	if !ok || zo == nil {
		return nil
	}
	return &phpObjectIterator{ctx: ctx, obj: zo, started: false}
}

// IsStmtAstDelegated reports whether r is a statement-shaped node
// the VM AST-delegates wholesale via OpTryFinally + OP_REFRESH_SLOTS.
func IsStmtAstDelegated(r phpv.Runnable) bool {
	switch r.(type) {
	case *runnableDeclareStrictTypes,
		*runnableDeclareTicks,
		*runTopLevelConst,
		*runEnumRegister:
		return true
	}
	return false
}

// UnwrapParens peels a `(expr)` wrapper from r. Returns r unchanged
// when r isn't a parenthesised expression. Used by the VM emitter so
// the surrounding context dispatches on the wrapped node directly.
func UnwrapParens(r phpv.Runnable) phpv.Runnable {
	if p, ok := r.(*runParentheses); ok {
		return p.r
	}
	return r
}

// CallHasSpecialArgs reports whether any of `args` is a named or
// spread argument — patterns the VM doesn't lower piecewise. Used
// by both the emitter (to AST-delegate the call) and IsSlotSafe
// (to mark the surrounding body unsafe).
func CallHasSpecialArgs(args []phpv.Runnable) bool {
	for _, a := range args {
		if _, ok := a.(phpv.NamedArgument); ok {
			return true
		}
		if _, ok := a.(phpv.SpreadArgument); ok {
			return true
		}
	}
	return false
}

// CallHasWritableArg reports whether any of `args` is a writable
// lvalue (variable, array element, object property, etc.). Used by
// the VM emitter to AST-delegate method calls whose receiver might
// declare a by-reference parameter — we can't statically resolve
// the method without polymorphism analysis, so when any arg could
// possibly need ref binding we route through the AST.
func CallHasWritableArg(args []phpv.Runnable) bool {
	for _, a := range args {
		switch a.(type) {
		case *runVariable, *runArrayAccess, *runObjectVar,
			*runClassStaticVarRef, *runClassStaticDynVarRef,
			*runVariableRef:
			return true
		}
	}
	return false
}

// IsIssetOrEmptyNode reports whether r is `isset(…)` or `empty(…)`.
// The VM emitter routes these through OpClassConst (push result).
// These don't write locals, so no slot refresh is needed.
func IsIssetOrEmptyNode(r phpv.Runnable) bool {
	switch r.(type) {
	case *runnableIsset, *runnableEmpty:
		return true
	}
	return false
}

// --- runConcat (string interpolation) ---------------------------------

// ConcatParts returns the list of expressions concatenated to form
// the result of `"foo $bar baz"` and similar interpolated strings.
// The emitter casts each part to string and emits N-1 OpConcat ops.
func (r runConcat) ConcatParts() []phpv.Runnable { return r }

// --- runnableTry / runnableCatch --------------------------------------

// TryNode exposes a try statement to the bytecode emitter.
type TryNode interface {
	TryBody() phpv.Runnable
	TryCatches() []TryCatchEntry
	TryFinally() phpv.Runnable
}

// TryCatchEntry exposes a single catch clause.
type TryCatchEntry interface {
	CatchTypes() []phpv.ZString
	CatchVarName() phpv.ZString
	CatchBody() phpv.Runnable
	CatchLoc() *phpv.Loc
}

// TryBody returns the try block.
func (r *runnableTry) TryBody() phpv.Runnable { return r.try }

// TryCatches returns the catch clauses.
func (r *runnableTry) TryCatches() []TryCatchEntry {
	out := make([]TryCatchEntry, len(r.catches))
	for i, c := range r.catches {
		out[i] = c
	}
	return out
}

// TryFinally returns the finally clause, or nil.
func (r *runnableTry) TryFinally() phpv.Runnable { return r.finally }

// CatchTypes returns the type-name list (the union in `catch (A | B $e)`).
func (r *runnableCatch) CatchTypes() []phpv.ZString { return r.typeNames }

// CatchVarName returns the bound variable name (without leading $).
// Empty when the catch has no variable: `catch (Exception)`.
func (r *runnableCatch) CatchVarName() phpv.ZString { return r.varname }

// CatchBody returns the catch block body.
func (r *runnableCatch) CatchBody() phpv.Runnable { return r.body }

// CatchLoc returns the source location of the catch.
func (r *runnableCatch) CatchLoc() *phpv.Loc { return r.l }

// --- runnableThrow ----------------------------------------------------

// ThrowValue returns the value expression being thrown.
func (r *runnableThrow) ThrowValue() phpv.Runnable { return r.v }

// ThrowLoc returns the source location.
func (r *runnableThrow) ThrowLoc() *phpv.Loc { return r.l }

// --- runNewObject -----------------------------------------------------

// NewObjectClassName returns the static class name when the
// `new ClassName(…)` form is used. Empty string means dynamic
// (`new $cls(…)`) — VM emitter falls back for that.
func (r *runNewObject) NewObjectClassName() phpv.ZString { return r.obj }

// NewObjectArgs returns the constructor argument expressions.
func (r *runNewObject) NewObjectArgs() phpv.Runnables { return r.newArg }

// NewObjectIsAnonymous reports whether this is `new class { … }`.
// Out of scope for the VM emitter.
func (r *runNewObject) NewObjectIsAnonymous() bool { return r.cl != nil }

// NewObjectLoc returns the source location.
func (r *runNewObject) NewObjectLoc() *phpv.Loc { return r.l }

// --- runObjectVar / runObjectFunc -------------------------------------

// objectVarReceiver returns the receiver expression of `$ref->name`.
func (r *runObjectVar) ObjectVarReceiver() phpv.Runnable { return r.ref }

// ObjectVarName returns the static property name.
func (r *runObjectVar) ObjectVarName() phpv.ZString { return r.varName }

// ObjectVarLoc returns the source location of the access.
func (r *runObjectVar) ObjectVarLoc() *phpv.Loc { return r.l }

// ObjectVarIsNullSafe reports whether this is a nullsafe chain
// (`$ref?->name`). Out of scope for the VM emitter for now.
func (r *runObjectVar) ObjectVarIsNullSafe() bool { return r.nullsafe || r.nullChain }

// ObjectFuncReceiver returns the receiver expression of
// `$ref->name(args...)`.
func (r *runObjectFunc) ObjectFuncReceiver() phpv.Runnable { return r.ref }

// ObjectFuncName returns the method name.
func (r *runObjectFunc) ObjectFuncName() phpv.ZString { return r.op }

// ObjectFuncArgs returns the call argument expressions.
func (r *runObjectFunc) ObjectFuncArgs() phpv.Runnables { return r.args }

// ObjectFuncLoc returns the source location of the call.
func (r *runObjectFunc) ObjectFuncLoc() *phpv.Loc { return r.l }

// ObjectFuncIsStatic reports whether this is a static-style call
// (`Foo::bar()` parsed via the property-call AST). Out of scope.
func (r *runObjectFunc) ObjectFuncIsStatic() bool { return r.static }

// ObjectFuncIsNullSafe reports nullsafe chain status.
func (r *runObjectFunc) ObjectFuncIsNullSafe() bool { return r.nullsafe || r.nullChain }

// --- runnableFunctionCallRef ($f(...) where $f is a variable) --------

// FuncCallRefExpr returns the expression evaluated to obtain the
// callable. Common case: a runVariable.
func (r *runnableFunctionCallRef) FuncCallRefExpr() phpv.Runnable { return r.name }

// FuncCallRefArgs returns the argument expressions.
func (r *runnableFunctionCallRef) FuncCallRefArgs() []phpv.Runnable { return r.args }

// FuncCallRefLoc returns the source location.
func (r *runnableFunctionCallRef) FuncCallRefLoc() *phpv.Loc { return r.l }

// --- runnableFunctionCall ---------------------------------------------

// FuncCallName returns the function name as written (or its global-
// fallback form). Empty if the call uses a dynamic name (variable
// function call), in which case the VM should fall back to AST.
func (r *runnableFunctionCall) FuncCallName() phpv.ZString { return r.name }

// FuncCallNsName returns the namespace-resolved fallback name for the
// callee (used by GetFunction when the local namespace doesn't define
// the function). May be empty.
func (r *runnableFunctionCall) FuncCallNsName() phpv.ZString { return r.nsName }

// FuncCallArgs returns the positional argument expressions.
func (r *runnableFunctionCall) FuncCallArgs() []phpv.Runnable { return r.args }

// FuncCallLoc returns the source location of the call.
func (r *runnableFunctionCall) FuncCallLoc() *phpv.Loc { return r.l }

// --- runArray / arrayEntry --------------------------------------------

// ArrayEntryNode exposes a single literal-array entry to out-of-package
// consumers (the bytecode emitter). spread entries (`...$expr`) and
// keyed entries (`k => v`) are distinguished via IsSpread / Key.
type ArrayEntryNode interface {
	EntryKey() phpv.Runnable   // nil for un-keyed entry
	EntryValue() phpv.Runnable // never nil
	EntrySpread() bool
}

// EntryKey returns the key expression (nil for un-keyed entries).
func (e *arrayEntry) EntryKey() phpv.Runnable { return e.k }

// EntryValue returns the value expression.
func (e *arrayEntry) EntryValue() phpv.Runnable { return e.v }

// EntrySpread reports whether this entry is `...$expr` (PHP 7.4+).
func (e *arrayEntry) EntrySpread() bool { return e.spread }

// ArrayEntries returns the literal-array entries as accessor nodes.
func (r *runArray) ArrayEntries() []ArrayEntryNode {
	out := make([]ArrayEntryNode, len(r.e))
	for i, e := range r.e {
		out[i] = e
	}
	return out
}

// ArrayLoc returns the source location of the array literal.
func (r *runArray) ArrayLoc() *phpv.Loc { return r.l }

// --- runArrayAccess ----------------------------------------------------

// ArrayAccessContainer returns the container expression (left side of [).
// Never nil for parsed source.
func (r *runArrayAccess) ArrayAccessContainer() phpv.Runnable { return r.value }

// ArrayAccessOffset returns the offset expression. nil for `$a[]`
// (the append form, only valid in write context).
func (r *runArrayAccess) ArrayAccessOffset() phpv.Runnable { return r.offset }

// ArrayAccessLoc returns the source location of the [...] access.
func (r *runArrayAccess) ArrayAccessLoc() *phpv.Loc { return r.l }

// ArrayAccessIsNullSafe reports whether this access is part of a
// nullsafe chain (`$obj?->arr[$k]`). Out of scope for the VM; emitter
// falls back to AST when true.
func (r *runArrayAccess) ArrayAccessIsNullSafe() bool { return r.nullChain }

// --- runConstant -------------------------------------------------------

// ConstantName returns the constant identifier as written in source.
// (e.g. "true", "PHP_INT_MAX"). Comparison with the well-known literal
// constants (true/false/null) is case-insensitive in PHP.
func (r *runConstant) ConstantName() string { return r.c }

// ConstantLoc returns the source location of the constant reference.
func (r *runConstant) ConstantLoc() *phpv.Loc { return r.l }

// --- runZVal -----------------------------------------------------------

// LiteralVal returns the wrapped Val for a runZVal (literal node).
func (r *runZVal) LiteralVal() phpv.Val { return r.v }

// LiteralLoc returns the source location of the literal.
func (r *runZVal) LiteralLoc() *phpv.Loc { return r.l }

// --- runVariable -------------------------------------------------------

// VariableName returns the variable name (without leading $).
func (r *runVariable) VariableName() phpv.ZString { return r.v }

// VariableLoc returns the source location of the variable read.
func (r *runVariable) VariableLoc() *phpv.Loc { return r.l }

// --- runOperator -------------------------------------------------------

// OperatorOp returns the tokenizer ItemType identifying the operator
// (e.g. tokenizer.Rune('+'), tokenizer.T_PLUS_EQUAL, tokenizer.T_INC).
func (r *runOperator) OperatorOp() tokenizer.ItemType { return r.op }

// OperatorA returns the left operand (LHS for assignment / write ops).
// May be nil for unary ops applied to the right operand.
func (r *runOperator) OperatorA() phpv.Runnable { return r.a }

// OperatorB returns the right operand. May be nil for postfix unary
// ops, in which case OperatorA() carries the operand.
func (r *runOperator) OperatorB() phpv.Runnable { return r.b }

// OperatorLoc returns the source location of the operator.
func (r *runOperator) OperatorLoc() *phpv.Loc { return r.l }

// OperatorIsWrite reports whether this op writes through its LHS
// (assignment, compound assignment, or ++/-- on the LHS).
func (r *runOperator) OperatorIsWrite() bool { return r.opD != nil && r.opD.write }

// OperatorIsCompound reports whether this is a compound-assign
// (+=, -=, .=, etc.) — write is true and there's an underlying op.
func (r *runOperator) OperatorIsCompound() bool {
	return r.opD != nil && r.opD.write && r.opD.op != nil
}

// --- runnableIf --------------------------------------------------------

// IfCond returns the condition expression.
func (r *runnableIf) IfCond() phpv.Runnable { return r.cond }

// IfYes returns the then-branch (or the value branch for a ternary).
func (r *runnableIf) IfYes() phpv.Runnable { return r.yes }

// IfNo returns the else-branch (may be nil for plain `if (...) {}`).
func (r *runnableIf) IfNo() phpv.Runnable { return r.no }

// IfLoc returns the source location of the if/ternary.
func (r *runnableIf) IfLoc() *phpv.Loc { return r.l }

// IfIsTernary reports whether this is a `?:` expression.
func (r *runnableIf) IfIsTernary() bool { return r.ternary }

// IfIsShortTernary reports whether this is a `cond ?: alt` (short
// ternary) where the value branch reuses the condition.
func (r *runnableIf) IfIsShortTernary() bool { return r.shortTernary }

// --- runnableFor -------------------------------------------------------

// ForStart returns the init expression list.
func (r *runnableFor) ForStart() phpv.Runnables { return r.start }

// ForCond returns the condition expression list (last result tested).
func (r *runnableFor) ForCond() phpv.Runnables { return r.cond }

// ForEach returns the per-iteration step expression list.
func (r *runnableFor) ForEach() phpv.Runnables { return r.each }

// ForCode returns the loop body.
func (r *runnableFor) ForCode() phpv.Runnable { return r.code }

// ForLoc returns the source location of the for statement.
func (r *runnableFor) ForLoc() *phpv.Loc { return r.l }

// --- runnableForeach ---------------------------------------------------

// ForeachSrc returns the source expression (the thing being iterated).
func (r *runnableForeach) ForeachSrc() phpv.Runnable { return r.src }

// ForeachKey returns the key target expression. nil when no `=> $k`.
// For supported (variable target) cases, this is a *runVariable.
func (r *runnableForeach) ForeachKey() phpv.Runnable { return r.k }

// ForeachValue returns the value target expression. Always non-nil.
func (r *runnableForeach) ForeachValue() phpv.Runnable { return r.v }

// ForeachCode returns the loop body.
func (r *runnableForeach) ForeachCode() phpv.Runnable { return r.code }

// ForeachIsRef reports whether the loop variable is by-reference
// (`foreach (... as &$v)`). The VM emitter falls back to AST when true.
func (r *runnableForeach) ForeachIsRef() bool { return r.ref }

// ForeachLoc returns the source location of the foreach statement.
func (r *runnableForeach) ForeachLoc() *phpv.Loc { return r.l }

// --- runnableWhile -----------------------------------------------------

// WhileCond returns the condition expression.
func (r *runnableWhile) WhileCond() phpv.Runnable { return r.cond }

// WhileCode returns the loop body.
func (r *runnableWhile) WhileCode() phpv.Runnable { return r.code }

// DoWhileCond returns the post-condition expression.
func (r *runnableDoWhile) DoWhileCond() phpv.Runnable { return r.cond }

// DoWhileCode returns the loop body.
func (r *runnableDoWhile) DoWhileCode() phpv.Runnable { return r.code }

// DoWhileLoc returns the source location of the do-while statement.
func (r *runnableDoWhile) DoWhileLoc() *phpv.Loc { return r.l }

// InstanceOfValue returns the LHS expression (the value being tested).
func (r *runInstanceOf) InstanceOfValue() phpv.Runnable { return r.v }

// InstanceOfStaticClass returns the static class name, or "" when the
// RHS is a dynamic-class expression accessed via InstanceOfClassVar.
func (r *runInstanceOf) InstanceOfStaticClass() phpv.ZString { return r.c }

// InstanceOfClassVar returns the dynamic class-name expression (a
// `$var instanceof $cls` form), or nil for static class names.
func (r *runInstanceOf) InstanceOfClassVar() phpv.Runnable { return r.classVar }

// CloneIsBasic reports whether r is a basic `clone $arg` (no
// withProperties, no spread). The VM lowers those natively; the
// extended PHP 8.5+ forms (`clone($x, $with)`, `clone(...$arr)`,
// named args) stay AST-delegated for now.
func (r *runnableClone) CloneIsBasic() bool {
	return r.with == nil && r.spread == nil && r.argNames[0] == "" && r.argNames[1] == "" && len(r.extra) == 0
}

// CloneArg returns the value being cloned (basic form only).
func (r *runnableClone) CloneArg() phpv.Runnable { return r.arg }

// WhileLoc returns the source location of the while statement.
func (r *runnableWhile) WhileLoc() *phpv.Loc { return r.l }

// --- runReturn ---------------------------------------------------------

// ReturnValue returns the return-value expression. nil for `return;`.
func (r *runReturn) ReturnValue() phpv.Runnable { return r.v }

// ReturnLoc returns the source location of the return statement.
func (r *runReturn) ReturnLoc() *phpv.Loc { return r.l }

// ReturnHasTypeHint reports whether the surrounding function declared
// a return type hint that may need coercion at the return site.
// VM-compiled returns must fall back to the AST when this is true,
// since the AST coercion path runs through compiler-internal logic.
func (r *runReturn) ReturnHasTypeHint() bool { return r.returnType != nil }

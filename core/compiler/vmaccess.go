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

// --- runConstant (user / namespaced constant reference) --------------

// ConstantNameAndFlags returns (name, noFallback, loc) so the VM can
// emit OP_LOAD_CONSTANT_BY_NAME without re-deriving them.
func ConstantNameAndFlags(r phpv.Runnable) (string, bool, *phpv.Loc) {
	t := r.(*runConstant)
	return t.c, t.noFallback, t.l
}

// IsRunConstantNode reports whether r is a *runConstant (user-defined
// or built-in constant reference); the VM emits OP_LOAD_CONSTANT_BY_NAME.
func IsRunConstantNode(r phpv.Runnable) bool {
	_, ok := r.(*runConstant)
	return ok
}

// --- runTopLevelConst -------------------------------------------------

// IsTopLevelConst reports whether r is a `const FOO = expr;` definition
// at file scope. The VM emits its value expression natively and a
// single OP_DEFINE_CONST that calls DefineTopLevelConst.
func IsTopLevelConst(r phpv.Runnable) bool {
	_, ok := r.(*runTopLevelConst)
	return ok
}

// TopLevelConstParts returns the const's static metadata so the VM can
// keep them in the const-pool / SubASTs slot. The value expression is
// returned separately because the VM emits it as native bytecode.
func TopLevelConstParts(r phpv.Runnable) (name phpv.ZString, val phpv.Runnable, attrs []*phpv.ZAttribute, loc *phpv.Loc) {
	t := r.(*runTopLevelConst)
	return t.name, t.val, t.attrs, t.l
}

// --- runNoDiscardStatement --------------------------------------------

// NoDiscardInner returns the wrapped statement. Used by the VM
// emitter to bracket the inner statement with OP_NODISCARD_ENTER /
// OP_NODISCARD_EXIT.
func (r *runNoDiscardStatement) NoDiscardInner() phpv.Runnable { return r.inner }

// IsNoDiscardNode reports whether r is a `#[NoDiscard]`-wrapped
// statement. The VM emitter brackets the inner with NoDiscardEnter /
// NoDiscardExit calls (via dedicated opcodes) so the call's
// LastCallable is checked at the right time.
func IsNoDiscardNode(r phpv.Runnable) bool {
	_, ok := r.(*runNoDiscardStatement)
	return ok
}

// --- runClassDynConst / runClassStaticObjRef / runClassStaticVarRef --

// IsClassConstNode reports whether r is one of the AST nodes for
// class-level constant / static-property / static-method-name access:
// `Foo::CONST`, `Foo::{$expr}`, `Foo::$bar`, `self::method`, etc.
// Visibility, interface/parent walking, CompileDelayed resolution and
// late-static-binding all live in the AST runners; the VM delegates
// via OP_CLASS_CONST.
func IsClassConstNode(r phpv.Runnable) bool {
	// All concrete forms — `Cls::CONST`, `Cls::{$expr}`, `Cls::$prop`,
	// `Cls::${$expr}`, `Cls::class`, `Cls::IDENT` — are now lowered
	// natively via their dedicated IsXxxNode predicates. Returning
	// false here means no expression falls through to OpClassConst's
	// AST delegation anymore for the class-const family.
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

// UnsetArgs returns the argument expressions of `unset(…)`. The VM
// uses this to decide between native emit (all args are simple
// variables) and AST delegation.
func UnsetArgs(r phpv.Runnable) []phpv.Runnable {
	t, ok := r.(*runnableUnset)
	if !ok {
		return nil
	}
	return []phpv.Runnable(t.args)
}

// UnsetAllSimple reports whether all args of an `unset(…)` are plain
// `$name` references — the form the VM lowers natively via OP_UNSET_LOCAL.
func UnsetAllSimple(r phpv.Runnable) bool {
	t, ok := r.(*runnableUnset)
	if !ok {
		return false
	}
	for _, a := range t.args {
		if !IsSimpleVariable(a) {
			return false
		}
	}
	return true
}

// IsUnsetSupportedArg reports whether an unset() argument shape has
// a native VM emit:
//   - simple variable ($x) → OP_UNSET_LOCAL
//   - array access on a simple-local container ($x[$k]) → OP_UNSET_DIM
//   - object property with a static (non-dollar-prefix) name and no
//     nullsafe marker ($obj->prop) → OP_UNSET_OBJ_PROP
func IsUnsetSupportedArg(r phpv.Runnable) bool {
	if IsSimpleVariable(r) {
		return true
	}
	if o, ok := r.(*runObjectVar); ok {
		// Reject dollar-prefix dyn-name and nullsafe forms — those
		// need separate opcodes or extra short-circuit logic. (Note:
		// compileUnset already rejects nullsafe outright, but we
		// double-check here for safety in case the parse path changes.)
		if o.nullsafe || o.nullChain {
			return false
		}
		if len(o.varName) > 0 && o.varName[0] == '$' {
			return false
		}
		// Receiver can be any emittable expression; we just need the
		// AST WriteValue pipeline's "unsetChain" semantics not to
		// matter, which holds for a single-level unset($obj->prop).
		// For nested $a->b->c, the inner runObjectVar.WriteValue
		// would set unsetChain on r.ref — we leave those to AST.
		if _, nested := o.ref.(*runObjectVar); nested {
			return false
		}
		return true
	}
	a, ok := r.(*runArrayAccess)
	if !ok || a.nullChain {
		return false
	}
	if a.offset == nil {
		return false
	}
	// Container must be a simple variable. Nested unsets like
	// unset($a[$k1][$k2]) still AST-delegate — the AST WriteValue
	// pipeline handles cached compound state we don't reproduce.
	return IsSimpleVariable(a.value)
}

// UnsetAllSupported reports whether all args of an `unset(…)` have a
// native-emittable shape.
func UnsetAllSupported(r phpv.Runnable) bool {
	t, ok := r.(*runnableUnset)
	if !ok {
		return false
	}
	for _, a := range t.args {
		if !IsUnsetSupportedArg(a) {
			return false
		}
	}
	return true
}

// IsAnonymousClassNode reports whether r is a `new class { … }`
// instantiation. The VM lowers value-context anon-class instantiations
// natively via OP_NEW_ANON_CLASS when all constructor args are pure
// value expressions; otherwise it AST-delegates so the by-ref param
// dance still works.
func IsAnonymousClassNode(r phpv.Runnable) bool {
	_, ok := r.(*runNewAnonymousClass)
	return ok
}

// AnonymousClassArgs returns the constructor argument expressions of
// a `new class { … }` instantiation, so the VM emitter can compile
// each natively.
func AnonymousClassArgs(r phpv.Runnable) []phpv.Runnable {
	return r.(*runNewAnonymousClass).constructorArgs
}

// AnonClassHasWritableArg reports whether any constructor arg is a
// CompoundWritable (array access, object prop, dyn name). Those
// expressions need the AST runner's by-ref auto-vivification dance;
// the VM keeps the AST path for them.
func AnonClassHasWritableArg(r phpv.Runnable) bool {
	t := r.(*runNewAnonymousClass)
	for _, a := range t.constructorArgs {
		if _, ok := a.(phpv.CompoundWritable); ok {
			return true
		}
	}
	return false
}

// EnsureAnonClassCompiled runs the per-AST-node class registration
// guard. Idempotent — safe to call on every instantiation.
func EnsureAnonClassCompiled(ctx phpv.Context, r phpv.Runnable) error {
	return r.(*runNewAnonymousClass).ensureCompiled(ctx)
}

// InstantiateAnonClass instantiates an already-compiled anonymous
// class with pre-evaluated value-context args. Used by the VM's
// OP_NEW_ANON_CLASS path.
func InstantiateAnonClass(ctx phpv.Context, r phpv.Runnable, args []*phpv.ZVal) (*phpv.ZVal, error) {
	t := r.(*runNewAnonymousClass)
	return InstantiateRegisteredAnonClass(ctx, t.class, args, nil, t.l)
}

// IsMatchNode reports whether r is a `match (…) { … }` expression.
// All matches are now lowered natively by the VM emitter, so this
// returns false unconditionally — kept as a hook in case a future
// match shape (e.g. exhaustive match on enum) needs to AST-delegate.
func IsMatchNode(r phpv.Runnable) bool {
	return false
}

// MatchArm exposes one arm of a `match (…) { … }` expression to the
// VM emitter: a list of strict-compare candidate values and the body
// expression to evaluate on a match.
type MatchArm interface {
	MatchArmConditions() []phpv.Runnable
	MatchArmBody() phpv.Runnable
}

// MatchNode exposes a `match` expression to the VM emitter.
type MatchNode interface {
	MatchCond() phpv.Runnable
	MatchArms() []MatchArm
	MatchDefaultBody() phpv.Runnable // nil if no default arm
	MatchLoc() *phpv.Loc
}

func (m *matchArm) MatchArmConditions() []phpv.Runnable { return m.conditions }
func (m *matchArm) MatchArmBody() phpv.Runnable         { return m.body }

func (r *runMatch) MatchCond() phpv.Runnable {
	return r.cond
}

func (r *runMatch) MatchArms() []MatchArm {
	out := make([]MatchArm, len(r.arms))
	for i, a := range r.arms {
		out[i] = a
	}
	return out
}

func (r *runMatch) MatchDefaultBody() phpv.Runnable {
	if r.def == nil {
		return nil
	}
	return r.def.body
}

func (r *runMatch) MatchLoc() *phpv.Loc { return r.l }

// IsSwitchNode reports whether r is a `switch (…) { … }` statement.
// All switches are now lowered natively by the VM emitter, so this
// returns false unconditionally — kept as a hook in case a future
// switch shape (e.g. with multi-level break that escapes the loops
// stack) needs to AST-delegate.
func IsSwitchNode(r phpv.Runnable) bool {
	return false
}

// SwitchBlock exposes one `case X: …` (or `default: …`) entry of a
// switch statement to the VM emitter.
type SwitchBlock interface {
	SwitchBlockCond() phpv.Runnable // nil for default
	SwitchBlockCode() phpv.Runnable
	SwitchBlockLoc() *phpv.Loc
}

// SwitchNode exposes a `switch` statement to the VM emitter.
type SwitchNode interface {
	SwitchCond() phpv.Runnable
	SwitchBlocks() []SwitchBlock
	SwitchLoc() *phpv.Loc
}

func (b *runSwitchBlock) SwitchBlockCond() phpv.Runnable { return b.cond }
func (b *runSwitchBlock) SwitchBlockCode() phpv.Runnable { return b.code }
func (b *runSwitchBlock) SwitchBlockLoc() *phpv.Loc      { return b.l }

func (r *runSwitch) SwitchCond() phpv.Runnable {
	return r.cond
}

func (r *runSwitch) SwitchBlocks() []SwitchBlock {
	out := make([]SwitchBlock, len(r.blocks))
	for i, b := range r.blocks {
		out[i] = b
	}
	return out
}

func (r *runSwitch) SwitchLoc() *phpv.Loc { return r.l }

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
// destructuring LHS. The VM emits OP_DESTRUCTURE_ASSIGN natively for
// these writes — the AST's runDestructure.WriteValue is reused as a
// helper to handle the recursive nested-list / keyed-entry dance.
func IsDestructureTarget(r phpv.Runnable) bool {
	_, ok := r.(*runDestructure)
	return ok
}

// AssignDestructure writes a value into a `list(…)` / `[…]` LHS,
// running the recursive nested-entry expansion that
// runDestructure.WriteValue implements. Shared by the AST runner
// and the VM's OP_DESTRUCTURE_ASSIGN.
func AssignDestructure(ctx phpv.Context, lhs phpv.Runnable, value *phpv.ZVal) error {
	return lhs.(*runDestructure).WriteValue(ctx, value)
}

// IsVariableRef reports whether r is a `$$name` / `${$expr}` variable-
// variable reference. The VM emits OP_VAR_VAR_READ natively for value
// reads; the write side still AST-delegates via runOperator's emit.
func IsVariableRef(r phpv.Runnable) bool {
	_, ok := r.(*runVariableRef)
	return ok
}

// VarVarNameExpr returns the name-producing sub-expression of `$$expr`
// / `${expr}`. The VM emits this to compute the variable name, then
// uses OP_VAR_VAR_READ to look up the actual variable.
func VarVarNameExpr(r phpv.Runnable) phpv.Runnable {
	return r.(*runVariableRef).v
}

// VarVarReadIsWriteParent reports whether r's Parent makes this `$$x`
// reference a write context (no "Undefined variable" warning even when
// the variable is missing). Used at emit time so the VM decides
// between warn vs no-warn OP_VAR_VAR_READ flags without re-implementing
// the AST runner's Parent-type switch.
func VarVarReadIsWriteParent(r phpv.Runnable) bool {
	v := r.(*runVariableRef)
	if v.Parent == nil {
		return true
	}
	switch t := v.Parent.(type) {
	case *runOperator:
		if t.opD.write {
			if t.opD.op != nil && t.a == v && t.op != tokenizer.T_CONCAT_EQUAL {
				return false
			}
			if (t.op == tokenizer.T_COALESCE_EQUAL || t.op == tokenizer.T_COALESCE) && t.b == v {
				return false
			}
			return true
		}
		return false
	case *runArrayAccess, *runDestructure, *runRef, *runnableUnset, *runGlobal:
		return true
	case *runObjectVar:
		return t.writeContext
	case phpv.Runnables:
		return true
	}
	return false
}

// IsValueExprAstDelegated reports whether r is a value-producing
// expression the VM AST-delegates wholesale via OpClassConst. The
// AST runner reads its operands from the FuncContext hashtable.
// IsSlotSafe rejects bodies containing any of these so the hashtable
// stays authoritative.
//
// Every previously-listed type now routes through a dedicated opcode:
//   - runnableClone (non-basic) → OP_CLONE_EXT via IsCloneExtNode
//   - runObjectDynVar (incl. nullChain) → OP_OBJECT_DYN_GET via
//     IsObjectDynVarReadNode
//   - runObjectDynFunc → OP_OBJECT_DYN_CALL via IsObjectDynFuncNode
//   - runRef → OP_CREATE_REF via IsRefNode
//
// IsValueExprAstDelegated now always returns false. It's retained as
// a single hook in case a future node type needs a generic-delegation
// fallback again.
func IsValueExprAstDelegated(r phpv.Runnable) bool {
	return false
}

// IsObjectDynFuncNode reports whether r is a dynamic-method-name call
// (`$obj->{$expr}(args)` or `Foo::{$expr}(args)`). The VM lowers it
// to OP_OBJECT_DYN_CALL which calls EvalObjectDynFunc.
func IsObjectDynFuncNode(r phpv.Runnable) bool {
	_, ok := r.(*runObjectDynFunc)
	return ok
}

// ObjectDynFuncReceiver returns the receiver expression of a
// dynamic-method-name call (`$obj->{$expr}(...)`). The VM emits this
// natively before OP_OBJECT_DYN_CALL.
func ObjectDynFuncReceiver(r phpv.Runnable) phpv.Runnable {
	return r.(*runObjectDynFunc).ObjectDynFuncReceiver()
}

// ObjectDynFuncNameExpr returns the expression that produces the
// method name. The VM emits this natively before OP_OBJECT_DYN_CALL.
func ObjectDynFuncNameExpr(r phpv.Runnable) phpv.Runnable {
	return r.(*runObjectDynFunc).ObjectDynFuncNameExpr()
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

// IsDeclareStrictTypesNode reports whether r is `declare(strict_types=1)`.
func IsDeclareStrictTypesNode(r phpv.Runnable) bool {
	_, ok := r.(*runnableDeclareStrictTypes)
	return ok
}

// IsFirstClassCallableNode reports whether r is `func(...)` syntax — the
// PHP 8.1 first-class callable for a free-function reference. Method
// and Closure variants keep AST-delegating.
func IsFirstClassCallableNode(r phpv.Runnable) bool {
	_, ok := r.(*runFirstClassCallable)
	return ok
}

// FirstClassCallableTarget returns the target expression (the function
// name, either a runConstant or a value expression).
func (r *runFirstClassCallable) FirstClassCallableTarget() phpv.Runnable { return r.target }

// IsFirstClassCloneCallableNode reports whether r is the `clone(...)`
// first-class callable (PHP 8.5+). Lowered to OP_FIRSTCLASS_CLONE.
func IsFirstClassCloneCallableNode(r phpv.Runnable) bool {
	_, ok := r.(*runFirstClassCloneCallable)
	return ok
}

// IsFirstClassMethodCallableNode reports whether r is a method
// first-class callable: `$obj->method(...)`, `Cls::method(...)`, or
// `$obj?->method(...)`. Lowered to OP_METHOD_FIRSTCLASS.
func IsFirstClassMethodCallableNode(r phpv.Runnable) bool {
	_, ok := r.(*runFirstClassMethodCallable)
	return ok
}

// MethodFirstClassReceiver returns the receiver expression (object
// instance, or class name for the static form).
func (r *runFirstClassMethodCallable) MethodFirstClassReceiver() phpv.Runnable { return r.ref }

// MethodFirstClassName returns the method name.
func (r *runFirstClassMethodCallable) MethodFirstClassName() phpv.ZString { return r.method }

// MethodFirstClassIsStatic reports the `Cls::m(...)` static form.
func (r *runFirstClassMethodCallable) MethodFirstClassIsStatic() bool { return r.static }

// MethodFirstClassIsNullsafe reports the `$x?->m(...)` form.
func (r *runFirstClassMethodCallable) MethodFirstClassIsNullsafe() bool { return r.nullsafe }

// IsFirstClassDynMethodCallableNode reports whether r is the
// dynamic-name form: `$obj->{expr}(...)`. Lowered to
// OP_DYN_METHOD_FIRSTCLASS.
func IsFirstClassDynMethodCallableNode(r phpv.Runnable) bool {
	_, ok := r.(*runFirstClassDynMethodCallable)
	return ok
}

// DynMethodFirstClassReceiver returns the receiver expression.
func (r *runFirstClassDynMethodCallable) DynMethodFirstClassReceiver() phpv.Runnable { return r.ref }

// DynMethodFirstClassNameExpr returns the method-name expression.
func (r *runFirstClassDynMethodCallable) DynMethodFirstClassNameExpr() phpv.Runnable { return r.nameExpr }

// IsObjectDynVarReadNode reports whether r is a `$obj->{$name}` or
// `$obj?->{$name}` value read (including the nullChain form, where an
// outer nullsafe operator has already poisoned the chain). The VM
// lowers all three to OP_OBJECT_DYN_GET with the nullsafe flag set
// when either nullsafe or nullChain is true — both want the same
// "obj == null → short-circuit to null" semantics.
func IsObjectDynVarReadNode(r phpv.Runnable) bool {
	_, ok := r.(*runObjectDynVar)
	return ok
}

// ObjectDynVarIsNullSafe reports whether the dyn-var access needs
// the null-short-circuit semantics — either the explicit `?->`
// nullsafe operator (`$obj?->{$name}`), or a propagated nullChain
// from an outer nullsafe element in the same chain.
func ObjectDynVarIsNullSafe(r phpv.Runnable) bool {
	n := r.(*runObjectDynVar)
	return n.nullsafe || n.nullChain
}

// ObjectDynVarReceiver returns the receiver expression.
func (r *runObjectDynVar) ObjectDynVarReceiver() phpv.Runnable { return r.ref }

// ObjectDynVarNameExpr returns the dynamic-name expression.
func (r *runObjectDynVar) ObjectDynVarNameExpr() phpv.Runnable { return r.nameExpr }

// GlobalVarEntry exposes one variable in a `global` declaration.
// Static returns the variable name (without leading $) when known
// statically; Dynamic returns the name expression for `global $$x`.
type GlobalVarEntry interface {
	GlobalStatic() phpv.ZString
	GlobalDynamic() phpv.Runnable
}

// IsGlobalDecl reports whether r is a `global $x, …` declaration. The
// VM lowers each entry to OP_GLOBAL_BIND (static-name) or eval+bind
// (dynamic). `static $y` declarations stay AST.
func IsGlobalDecl(r phpv.Runnable) bool {
	_, ok := r.(*runGlobal)
	return ok
}

// GlobalEntries returns the list of variables declared in `global`.
func GlobalEntries(r phpv.Runnable) []GlobalVarEntry {
	g, ok := r.(*runGlobal)
	if !ok {
		return nil
	}
	out := make([]GlobalVarEntry, len(g.vars))
	for i := range g.vars {
		out[i] = &g.vars[i]
	}
	return out
}

func (v *globalVar) GlobalStatic() phpv.ZString { return v.static }
func (v *globalVar) GlobalDynamic() phpv.Runnable {
	return v.dynamic
}

// IsClassDynConstNode reports whether r is `Cls::CONST` /
// `$obj::CONST` / `Cls::{$name}`. The VM lowers to OP_CLASS_DYN_CONST.
func IsClassDynConstNode(r phpv.Runnable) bool {
	_, ok := r.(*runClassDynConst)
	return ok
}

// ClassDynConstClassExpr returns the class-source expression
// (`Cls` literal, `$obj`, `self`, `parent`, `static`, …).
func (r *runClassDynConst) ClassDynConstClassExpr() phpv.Runnable { return r.className }

// ClassDynConstNameExpr returns the const-name expression (constant
// for `::CONST`, full expression for `::{$expr}`).
func (r *runClassDynConst) ClassDynConstNameExpr() phpv.Runnable { return r.nameExpr }

// IsClassStaticVarReadNode reports whether r is a `Cls::$prop` read
// with a statically-known property name. The VM lowers it to
// OP_CLASS_STATIC_GET.
func IsClassStaticVarReadNode(r phpv.Runnable) bool {
	_, ok := r.(*runClassStaticVarRef)
	return ok
}

// ClassStaticVarReadClassExpr returns the class-source expression.
func (r *runClassStaticVarRef) ClassStaticVarReadClassExpr() phpv.Runnable { return r.className }

// ClassStaticVarReadName returns the static property name.
func (r *runClassStaticVarRef) ClassStaticVarReadName() phpv.ZString { return r.varName }

// ClassStaticVarReadLoc returns the source location.
func (r *runClassStaticVarRef) ClassStaticVarReadLoc() *phpv.Loc { return r.l }

// IsClassStaticObjRefNode reports whether r is `Cls::IDENT` (literal
// const-name class const fetch). Lowered to OP_CLASS_STATIC_OBJ_REF.
func IsClassStaticObjRefNode(r phpv.Runnable) bool {
	_, ok := r.(*runClassStaticObjRef)
	return ok
}

// ClassStaticObjRefClassExpr returns the class-source expression.
func (r *runClassStaticObjRef) ClassStaticObjRefClassExpr() phpv.Runnable { return r.className }

// ClassStaticObjRefName returns the literal identifier name.
func (r *runClassStaticObjRef) ClassStaticObjRefName() phpv.ZString { return r.objName }

// ClassStaticObjRefLoc returns the source location.
func (r *runClassStaticObjRef) ClassStaticObjRefLoc() *phpv.Loc { return r.l }

// IsClassStaticDynVarReadNode reports whether r is a `Cls::${$name}`
// dyn-name static-prop read. The VM lowers to OP_CLASS_STATIC_DYN_GET.
func IsClassStaticDynVarReadNode(r phpv.Runnable) bool {
	_, ok := r.(*runClassStaticDynVarRef)
	return ok
}

// ClassStaticDynVarReadClassExpr returns the class-source expression.
func (r *runClassStaticDynVarRef) ClassStaticDynVarReadClassExpr() phpv.Runnable {
	return r.className
}

// ClassStaticDynVarReadNameExpr returns the name expression.
func (r *runClassStaticDynVarRef) ClassStaticDynVarReadNameExpr() phpv.Runnable {
	return r.nameExpr
}

// IsDeclareTicksNode reports whether r is `declare(ticks=N) { … }`.
// The VM emits the body normally and inserts OP_CALL_TICK_FUNCTIONS
// after every N statements.
func IsDeclareTicksNode(r phpv.Runnable) bool {
	_, ok := r.(*runnableDeclareTicks)
	return ok
}

// DeclareTicksBody returns the wrapped body.
func (r *runnableDeclareTicks) DeclareTicksBody() phpv.Runnable { return r.body }

// DeclareTicksN returns the tick period.
func (r *runnableDeclareTicks) DeclareTicksN() int64 { return r.ticks }

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
// All statement-shaped delegations are now lowered natively; this
// returns false unconditionally.
func IsStmtAstDelegated(r phpv.Runnable) bool {
	return false
}

// IsEnumRegisterNode reports whether r is a `runEnumRegister`
// (enum declaration). The VM emits a single OP_REGISTER_ENUM which
// invokes the AST node's full registration + validation flow via
// the shared RegisterEnum helper.
func IsEnumRegisterNode(r phpv.Runnable) bool {
	_, ok := r.(*runEnumRegister)
	return ok
}

// RegisterEnum runs the enum-specific registration + Compile +
// post-validation dance for a *runEnumRegister AST node. Both the
// AST runner and the VM (OP_REGISTER_ENUM) call this.
func RegisterEnum(ctx phpv.Context, r phpv.Runnable) error {
	t := r.(*runEnumRegister)
	return t.register(ctx)
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

// IssetArgs returns the argument expressions of `isset(…)`. The VM
// uses this to decide between native emit (all args are simple
// variables) and AST delegation.
func IssetArgs(r phpv.Runnable) []phpv.Runnable {
	t, ok := r.(*runnableIsset)
	if !ok {
		return nil
	}
	return []phpv.Runnable(t.args)
}

// EmptyArg returns the single argument expression of `empty(…)`.
func EmptyArg(r phpv.Runnable) phpv.Runnable {
	t, ok := r.(*runnableEmpty)
	if !ok {
		return nil
	}
	return t.arg
}

// IsSimpleVariable reports whether r is a plain `$name` reference
// (no array access, no object dereference, no var-var). The VM uses
// this to decide whether an isset/empty argument can be lowered to a
// slot read.
func IsSimpleVariable(r phpv.Runnable) bool {
	_, ok := r.(*runVariable)
	return ok
}

// IssetEmptyAllSimple reports whether the given isset/empty node has
// only simple-variable arguments — the form the VM lowers natively
// via OP_ISSET_LOCAL / OP_EMPTY_LOCAL.
func IssetEmptyAllSimple(r phpv.Runnable) bool {
	switch t := r.(type) {
	case *runnableIsset:
		for _, a := range t.args {
			if !IsSimpleVariable(a) {
				return false
			}
		}
		return true
	case *runnableEmpty:
		return IsSimpleVariable(t.arg)
	}
	return false
}

// IsIssetSupportedArg reports whether an isset/empty argument shape
// has a native VM emit. Currently:
//   - simple variable ($x)
//   - array-access with a non-nullsafe simple-shape container
//     ($x[$k], $x[k], also nested $x[$k1][$k2])
//   - object property with a static (non-dollar-prefix) name and no
//     nullsafe marker ($obj->prop). Receiver itself can be any
//     emittable expression.
func IsIssetSupportedArg(r phpv.Runnable) bool {
	if IsSimpleVariable(r) {
		return true
	}
	if o, ok := r.(*runObjectVar); ok {
		// Reject dollar-prefix dyn-name and nullsafe forms — those
		// need separate opcodes or extra short-circuit logic.
		if o.nullsafe || o.nullChain {
			return false
		}
		if len(o.varName) > 0 && o.varName[0] == '$' {
			return false
		}
		// Receiver chain must also be a supported isset shape so
		// emitIssetContainerRead can recurse without falling back.
		return IsIssetSupportedArg(o.ref)
	}
	if _, ok := r.(*runClassStaticVarRef); ok {
		// `Foo::$bar` — class source is evaluated by emitExpr;
		// EvalIssetStaticProp / EvalEmptyStaticProp handle the
		// downstream visibility/declared/non-null checks.
		return true
	}
	a, ok := r.(*runArrayAccess)
	if !ok || a.nullChain {
		return false
	}
	if a.offset == nil {
		// `$x[]` — invalid in read context.
		return false
	}
	// Recurse: the container itself must be a supported shape.
	return IsIssetSupportedArg(a.value)
}

// IssetEmptyAllSupported reports whether all args of an isset/empty
// have a native-emittable shape (IsIssetSupportedArg).
func IssetEmptyAllSupported(r phpv.Runnable) bool {
	switch t := r.(type) {
	case *runnableIsset:
		for _, a := range t.args {
			if !IsIssetSupportedArg(a) {
				return false
			}
		}
		return true
	case *runnableEmpty:
		return IsIssetSupportedArg(t.arg)
	}
	return false
}

// IsArrayAccessNode reports whether r is a `$container[$offset]`
// array access (non-nullsafe). The VM emitter uses this in the
// isset/empty per-arg dispatch.
func IsArrayAccessNode(r phpv.Runnable) bool {
	a, ok := r.(*runArrayAccess)
	return ok && !a.nullChain
}

// ArrayAccessParts returns the container/offset of an array-access
// node. Caller must verify with IsArrayAccessNode first.
func ArrayAccessParts(r phpv.Runnable) (container, offset phpv.Runnable) {
	a := r.(*runArrayAccess)
	return a.value, a.offset
}

// SimpleVariableName returns the name of a plain `$name` reference.
// Caller must verify the runnable is a simple variable first.
func SimpleVariableName(r phpv.Runnable) phpv.ZString {
	return r.(*runVariable).v
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

// --- runObjectDynFunc ($obj->{$expr}(args)) ---------------------------

// ObjectDynFuncReceiver returns the receiver expression.
func (r *runObjectDynFunc) ObjectDynFuncReceiver() phpv.Runnable { return r.ref }

// ObjectDynFuncNameExpr returns the expression that produces the method name.
func (r *runObjectDynFunc) ObjectDynFuncNameExpr() phpv.Runnable { return r.nameExpr }

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

// ReturnTypeHint returns the function's return type hint for the
// surrounding function. nil when none was declared. Used by the VM
// emitter's typed-return lowering.
func (r *runReturn) ReturnTypeHint() *phpv.TypeHint { return r.returnType }

// ReturnTypeOfNode extracts the return type from a *runReturn AST
// node. Used by the VM's OP_COERCE_RETURN dispatch.
func ReturnTypeOfNode(r phpv.Runnable) *phpv.TypeHint {
	t, ok := r.(*runReturn)
	if !ok {
		return nil
	}
	return t.returnType
}

// ReturnValueIsRefTarget reports whether the return value expression
// is a shape (object property, array access, static-prop) that needs
// the AST runner's writeContext setup when the surrounding function
// returns by reference. Used by the VM emitter to decide whether the
// native typed-return path is safe.
func ReturnValueIsRefTarget(r phpv.Runnable) bool {
	switch r.(type) {
	case *runObjectVar, *runObjectDynVar, *runArrayAccess, *runClassStaticVarRef:
		return true
	}
	return false
}

// ReturnHasTypeHint reports whether the surrounding function declared
// a return type hint that may need coercion at the return site.
// VM-compiled returns must fall back to the AST when this is true,
// since the AST coercion path runs through compiler-internal logic.
func (r *runReturn) ReturnHasTypeHint() bool { return r.returnType != nil }

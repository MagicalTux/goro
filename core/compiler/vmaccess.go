package compiler

import (
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// TryBuildVMClosureBody is an optional hook installed by the
// core/vm/vmcompiler package at init time. When set, the compiler
// invokes it after building each closure body (`function foo() { ... }`,
// closures, arrow fns) and, if it returns a non-nil Runnable, replaces
// the AST body with the VM-backed runner.
//
// Returning nil means "VM compilation failed or was declined; keep the
// AST body". Implementations must be safe to call concurrently and
// must not mutate the input Runnable.
//
// Set to nil (the zero value) when the vmcompiler is not linked or has
// disabled itself via env var. Compiler treats nil as "VM disabled".
var TryBuildVMClosureBody func(name phpv.ZString, src *phpv.Loc, body phpv.Runnable) phpv.Runnable

// TryBuildVMScript is the equivalent hook for top-level scripts. It
// gets the full Runnables and may return a wrapping VM runner.
var TryBuildVMScript func(src *phpv.Loc, body phpv.Runnable) phpv.Runnable

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

// --- runnableWhile -----------------------------------------------------

// WhileCond returns the condition expression.
func (r *runnableWhile) WhileCond() phpv.Runnable { return r.cond }

// WhileCode returns the loop body.
func (r *runnableWhile) WhileCode() phpv.Runnable { return r.code }

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

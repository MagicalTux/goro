package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// arrayLiteralNode matches *compiler.runArray.
type arrayLiteralNode interface {
	ArrayEntries() []compiler.ArrayEntryNode
	ArrayLoc() *phpv.Loc
}

// arrayAccessNode matches *compiler.runArrayAccess.
type arrayAccessNode interface {
	ArrayAccessContainer() phpv.Runnable
	ArrayAccessOffset() phpv.Runnable
	ArrayAccessLoc() *phpv.Loc
	ArrayAccessIsNullSafe() bool
}

// emitArrayLiteral lowers `[…]`. Spread entries (…$expr) are routed
// through the AST runner — they need iterator semantics with key
// coercion and Traversable support that's involved enough we'd
// rather reuse the AST implementation than re-derive in bytecode.
func (e *emitter) emitArrayLiteral(n arrayLiteralNode) error {
	for _, entry := range n.ArrayEntries() {
		if entry.EntrySpread() {
			raw, ok := any(n).(phpv.Runnable)
			if !ok {
				return unsupportedf("spread in array literal: cannot retrieve raw Runnable")
			}
			idx := e.astIndex(raw)
			e.emit(vm.OpClassConst, idx, 0, 0)
			e.pushStack(1)
			// No OP_REFRESH_SLOTS: array literal evaluation reads
			// locals but doesn't write them.
			return nil
		}
	}

	e.emit(vm.OpNewArray, 0, 0, 0)
	e.pushStack(1)

	for _, entry := range n.ArrayEntries() {
		if entry.EntryKey() != nil {
			if err := e.withSubexpr(func() error { return e.emitExpr(entry.EntryKey()) }); err != nil {
				return err
			}
			if err := e.withSubexpr(func() error { return e.emitExpr(entry.EntryValue()) }); err != nil {
				return err
			}
			e.emit(vm.OpArrayInitKeyed, 0, 0, 0)
			e.popStack(2) // pops k+v, leaves array on top
		} else {
			if err := e.withSubexpr(func() error { return e.emitExpr(entry.EntryValue()) }); err != nil {
				return err
			}
			e.emit(vm.OpArrayInitAppend, 0, 0, 0)
			e.popStack(1) // pops v, leaves array on top
		}
	}
	return nil
}

// emitArrayAccessRead lowers `$container[$offset]` in read context.
// Falls back via ErrUnsupported when the container isn't simple
// enough to dispatch via OP_ARRAY_GET (object/string offsets, etc.).
func (e *emitter) emitArrayAccessRead(n arrayAccessNode) error {
	if n.ArrayAccessIsNullSafe() {
		return unsupportedf("nullsafe array access")
	}
	if n.ArrayAccessOffset() == nil {
		// `$a[]` in read context is invalid PHP (already errors at
		// compile/run time). Defer to the AST so the same error
		// surfaces with the expected message.
		return unsupportedf("read of `$a[]` (no offset)")
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ArrayAccessContainer()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(n.ArrayAccessOffset()) }); err != nil {
		return err
	}
	e.emit(vm.OpArrayGet, 0, 0, 0)
	e.popStack(1) // pops 2 (container, offset), pushes 1
	return nil
}

// emitArrayAssignToLocal handles `$local[offset] = value` and
// `$local[] = value` assignment (statement context only). The LHS
// must be a runArrayAccess whose container is the variable directly
// — anything more complex falls back.
func (e *emitter) emitArrayAssignToLocal(lhs arrayAccessNode, rhs phpv.Runnable, stmtCtx bool) error {
	if !stmtCtx {
		return unsupportedf("array element assignment in non-statement context")
	}
	if lhs.ArrayAccessIsNullSafe() {
		return unsupportedf("nullsafe array assignment")
	}
	// Container must be a simple variable.
	v, ok := lhs.ArrayAccessContainer().(variableNode)
	if !ok {
		return unsupportedf("array assign to non-local container %T", lhs.ArrayAccessContainer())
	}
	idx := e.localIndex(v.VariableName())

	if lhs.ArrayAccessOffset() == nil {
		// Append form: `$local[] = $rhs`
		if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
			return err
		}
		e.emit(vm.OpArrayAppendLocal, idx, 0, 0)
		e.popStack(1)
		return nil
	}
	// Indexed form: `$local[$k] = $rhs`
	if err := e.withSubexpr(func() error { return e.emitExpr(lhs.ArrayAccessOffset()) }); err != nil {
		return err
	}
	if err := e.withSubexpr(func() error { return e.emitExpr(rhs) }); err != nil {
		return err
	}
	e.emit(vm.OpArraySetLocal, idx, 0, 0)
	e.popStack(2)
	return nil
}

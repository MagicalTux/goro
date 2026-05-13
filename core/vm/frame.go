package vm

import "github.com/KarpelesLab/goro/core/phpv"

// Slot is the type of a single VM stack cell.
//
// MVP: alias for *phpv.ZVal so the VM's contract matches the AST and
// the operator helpers in core/compiler accept slots verbatim.
//
// Migration seam: when we move to unboxed values, Slot becomes a small
// struct (type tag + payload). Only the helpers in ops_*.go need to
// box/unbox; opcode encoding and the dispatch loop are unaffected.
type Slot = *phpv.ZVal

// Frame is a single VM activation record. One per executing Function.
type Frame struct {
	fn    *Function
	pc    uint32
	stack []Slot
	sp    int // points one past the topmost valid slot

	// iters holds active foreach iterators, pushed by OP_FOREACH_INIT
	// and popped by OP_FOREACH_UNWIND (or OP_FOREACH_STEP when the
	// iterator is exhausted). Nested foreaches stack here.
	iters []phpv.ZIterator

	// locals is a fixed-size slot array indexed by Function.Locals
	// position. It mirrors the FuncContext hashtable for the VM's
	// fast path: reads come straight from the slot (no map lookup);
	// writes update the slot AND the hashtable so external callers
	// (extract/compact/builtins reading the FuncContext) still see
	// fresh values.
	//
	// A nil slot means "variable is undefined" (just like an absent
	// hashtable entry). The dispatcher's OP_LOAD_LOCAL_OR_WARN treats
	// it as such.
	locals []*phpv.ZVal
}

// refreshSlots re-reads each local from the FuncContext hashtable
// into the slot cache. Called after AST delegation (e.g. OP_TRY_FINALLY,
// OP_CLASS_CONST when CompileDelayed runs) so any locals the AST
// modified via ctx.OffsetSet are picked up by subsequent slot reads.
func (f *Frame) refreshSlots(ctx phpv.Context) {
	for i, name := range f.fn.Locals {
		if v, exists, _ := ctx.OffsetCheck(ctx, name); exists && v != nil {
			f.locals[i] = v
		} else {
			f.locals[i] = nil
		}
	}
}

// syncSlots pushes every non-nil slot into the FuncContext hashtable.
// Called before AST delegation in slot-only bodies so the AST can read
// fresh local values (the slot-only optimization normally skips the
// hashtable mirror on writes). Pair with refreshSlots after the AST run.
//
// Skipping nil slots: a nil slot means the local is "undefined" — we
// don't want to OffsetSet ZNULL which would create a defined entry.
func (f *Frame) syncSlots(ctx phpv.Context) error {
	for i, v := range f.locals {
		if v == nil {
			continue
		}
		if err := ctx.OffsetSet(ctx, f.fn.Locals[i], v); err != nil {
			return err
		}
	}
	return nil
}

// storeLocal writes v into both the slot cache and the FuncContext
// hashtable (the latter is skipped when fn.SlotOnly is set).
//
// Cached singleton ZVals (small ints, true/false, …) are duplicated
// first — same policy as ZHashTable.SetString — so any later in-place
// mutation (e.g. DoInc) lands in a fresh ZVal rather than panicking
// on the cached one.
//
// SlotOnly skips OffsetSet: a sizeable perf win on write-heavy loops.
// The emitter only sets it for functions whose bodies are statically
// confirmed not to depend on the FuncContext hashtable
// (no extract/compact/$$x/etc.).
func (f *Frame) storeLocal(ctx phpv.Context, idx uint16, v *phpv.ZVal) error {
	if v == nil {
		f.locals[idx] = nil
		if f.fn.SlotOnly {
			return nil
		}
		return ctx.OffsetSet(ctx, f.fn.Locals[idx], nil)
	}
	if v.IsCached() {
		v = phpv.NewZVal(v.Value())
	}
	f.locals[idx] = v
	if f.fn.SlotOnly {
		return nil
	}
	return ctx.OffsetSet(ctx, f.fn.Locals[idx], v)
}

// push grows the stack by one and stores v at the new top.
func (f *Frame) push(v Slot) {
	f.stack[f.sp] = v
	f.sp++
}

// pop returns the top of the stack and shrinks by one. The slot is
// nilled out so the GC can collect referenced values once the frame
// stops touching them.
func (f *Frame) pop() Slot {
	f.sp--
	v := f.stack[f.sp]
	f.stack[f.sp] = nil
	return v
}

// peek returns the top of the stack without popping.
func (f *Frame) peek() Slot {
	return f.stack[f.sp-1]
}

// peekAt returns the slot offset cells below the top (peekAt(0) == peek()).
func (f *Frame) peekAt(offset int) Slot {
	return f.stack[f.sp-1-offset]
}

// replaceTop overwrites the top slot. Equivalent to pop+push but avoids
// the nil/restore round-trip.
func (f *Frame) replaceTop(v Slot) {
	f.stack[f.sp-1] = v
}

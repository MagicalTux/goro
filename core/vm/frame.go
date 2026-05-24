package vm

import (
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
)

// pendingKind tags the action being deferred while a finally body runs.
type pendingKind uint8

const (
	pendingNone pendingKind = iota
	// pendingReturn: the original OP_RET / OP_RET_NULL was intercepted
	// by an enclosing finally. pending.val holds the return value; the
	// finally runs first, then OP_FINALLY_END either chains into an
	// outer finally or performs the actual return.
	pendingReturn
	// pendingThrow: a throw with no matching catch (or thrown inside a
	// catch body) was intercepted by an enclosing finally. pending.err
	// holds the *phperr.PhpThrow; OP_FINALLY_END re-raises it.
	pendingThrow
)

// pendingControl is one finally body's deferred control action. Each
// TryHandler with HasFinally gets its own slot in Frame.handlerPending,
// indexed by the handler's position in Function.TryHandlers — so a
// throw caught by a try-catch nested inside an outer finally body
// doesn't disturb the outer's pending entry (bug65784). The slot is
// set when control is routed INTO this handler's finally (via OpRet,
// OpRetNull, or dispatchTryHandler's uncaught-throw route) and
// read+cleared at OP_FINALLY_END for that same handler. A throw that
// escapes a finally body without reaching its OP_FINALLY_END leaves
// the slot stale; the frame is reset on acquire so it can't leak into
// later invocations.
type pendingControl struct {
	kind pendingKind
	val  *phpv.ZVal
	err  *phperr.PhpThrow
}

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

	// handlerPending is a per-TryHandler slot for a finally body's
	// deferred control action (see pendingControl). Sized to match
	// len(fn.TryHandlers) at acquire time so handler index is a direct
	// lookup. A handler without HasFinally never has its slot read.
	handlerPending []pendingControl
}

// findEnclosingFinally returns the handler index and pointer for the
// innermost finally that protects PC `pc` — i.e. the handler with
// HasFinally set whose [Start, FinallyPC) region contains pc. Used by
// OP_RET / OP_RET_NULL and by OP_FINALLY_END's chain-to-outer logic to
// decide whether a return must route through a finally first.
//
// Returns (-1, nil, false) when no enclosing finally covers pc.
func (f *Frame) findEnclosingFinally(pc uint32) (int, *TryHandler, bool) {
	handlers := f.fn.TryHandlers
	// Innermost-first: handlers are appended in emit order, and a
	// nested try's handler is appended before its outer's (the inner
	// emitTry completes during the outer's body emission). So a
	// forward scan returns the innermost match first.
	for i := range handlers {
		h := &handlers[i]
		if !h.HasFinally {
			continue
		}
		if pc >= h.Start && pc < h.FinallyPC {
			return i, h, true
		}
	}
	return -1, nil, false
}

// resetStackTo trims the value stack down to depth `n`, niling out the
// dropped slots so the GC can reclaim them. Used when unwinding into a
// catch or finally body.
func (f *Frame) resetStackTo(n int) {
	for f.sp > n {
		f.sp--
		f.stack[f.sp] = nil
	}
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
	// Array values get a COW Dup on assignment so subsequent
	// mutations through one variable don't surprise the other
	// (PHP value semantics for arrays). Matches the AST's
	// ZVal.ZVal() path in runOperator's `=` branch. If the source
	// is a reference to an array (a by-ref param or foreach &\$v),
	// dereference first then Dup — plain assign should detach.
	if v.GetType() == phpv.ZtArray {
		if v.IsRef() {
			// Follow the ref chain to the underlying ZVal, then Dup.
			v = v.Nude().Dup()
		} else {
			v = v.Dup()
		}
	} else if !v.IsRef() {
		// Object/string/scalar slots: allocate a fresh ZVal wrapper
		// so the new slot doesn't share the ZVal pointer with its
		// CoW siblings. Without this, a by-ref builtin (e.g.
		// mb_convert_variables) that calls z.Set on \$b mutates the
		// shared wrapper and \$a sees the change too (bug26639).
		// Strings/ints stay byte-identical because we only allocate
		// a new wrapper around the same underlying Val.
		v = v.ZVal()
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

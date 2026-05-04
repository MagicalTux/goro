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

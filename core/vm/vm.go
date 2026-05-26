package vm

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// ErrUnknownOp is returned when the dispatcher encounters an opcode it
// doesn't implement yet. Surfaces emitter bugs early rather than silent
// wrong behaviour.
var ErrUnknownOp = errors.New("vm: unknown opcode")

// Run executes the given Function in ctx. The returned ZVal is the
// value left on top of the stack by OpRet (or null for OpRetNull).
//
// At top-level, fn is the script body. For function calls the args are
// pre-bound into the surrounding context; this entry point doesn't
// touch arguments.
func Run(ctx phpv.Context, fn *Function) (*phpv.ZVal, error) {
	f := acquireFrame(fn)
	defer releaseFrame(f)
	// Pre-load any locals that already exist in the FuncContext (most
	// commonly: parameters bound by ZClosure.callBody before us).
	// Use OffsetCheck to distinguish "exists" from "missing" — plain
	// OffsetGet returns a fresh ZNULL.ZVal() for missing keys, which
	// would defeat the OP_LOAD_LOCAL_OR_WARN nil-check.
	for i, name := range fn.Locals {
		if v, exists, _ := ctx.OffsetCheck(ctx, name); exists && v != nil {
			f.locals[i] = v
		}
	}
	return f.exec(ctx)
}

// framePool reuses Frame structs and their backing slices across calls.
// The stack and locals slices are reused when their lengths match the
// next caller's needs (the common case for recursive calls into the
// same function).
var framePool = sync.Pool{
	New: func() any { return &Frame{} },
}

func acquireFrame(fn *Function) *Frame {
	f := framePool.Get().(*Frame)
	f.fn = fn
	f.pc = 0
	f.sp = 0
	if cap(f.stack) >= fn.MaxStack {
		f.stack = f.stack[:fn.MaxStack]
	} else {
		f.stack = make([]Slot, fn.MaxStack)
	}
	if cap(f.locals) >= len(fn.Locals) {
		f.locals = f.locals[:len(fn.Locals)]
		for i := range f.locals {
			f.locals[i] = nil
		}
	} else {
		f.locals = make([]*phpv.ZVal, len(fn.Locals))
	}
	if n := len(fn.TryHandlers); n > 0 {
		if cap(f.handlerPending) >= n {
			f.handlerPending = f.handlerPending[:n]
			for i := range f.handlerPending {
				f.handlerPending[i] = pendingControl{}
			}
		} else {
			f.handlerPending = make([]pendingControl, n)
		}
	} else {
		f.handlerPending = f.handlerPending[:0]
	}
	return f
}

func releaseFrame(f *Frame) {
	// Clear references so the GC can reclaim ZVals/iterators that
	// were sitting in the slots. Keep the backing arrays so the next
	// caller can reuse them.
	for i := range f.stack[:f.sp] {
		f.stack[i] = nil
	}
	f.iters = f.iters[:0]
	f.fn = nil
	framePool.Put(f)
}

// exec is the dispatch loop. Returns whatever value OpRet popped, or
// nil if execution falls off the end without a return (top-level
// scripts).
//
// Internally, each opcode-step that produces an error returns it via
// the inner runUntilError loop. The outer wrapper here checks for
// PhpThrow and looks up a matching try-handler. On a match it sets pc
// to the catch body and continues the inner loop; otherwise the error
// propagates out (after the deferred loc-wrap).
func (f *Frame) exec(ctx phpv.Context) (res *phpv.ZVal, err error) {
	defer func() {
		if err == nil {
			return
		}
		if _, isThrow := err.(*phperr.PhpThrow); isThrow {
			return
		}
		if _, isReturn := err.(*phperr.PhpReturn); isReturn {
			return
		}
		if _, isBreak := err.(*phperr.PhpBreak); isBreak {
			return
		}
		if _, isContinue := err.(*phperr.PhpContinue); isContinue {
			return
		}
		pc := f.pc
		if pc > 0 {
			pc--
		}
		if loc := f.fn.LocAt(pc); loc != nil {
			err = loc.Error(ctx, err)
		}
	}()

	for {
		retVal, finished, ierr := f.runUntilError(ctx)
		if ierr != nil {
			// Loop here so a destructor exception during catch-var
			// binding (PHP bug 53511 semantics) re-enters the search
			// for an outer handler.
			for {
				t, ok := ierr.(*phperr.PhpThrow)
				if !ok {
					break
				}
				handled, newErr := f.dispatchTryHandler(ctx, t)
				if newErr != nil {
					ierr = newErr
					continue
				}
				if handled {
					ierr = nil
					break
				}
				break
			}
			if ierr != nil {
				return nil, ierr
			}
			continue
		}
		if finished {
			return retVal, nil
		}
		return retVal, nil
	}
}

// runUntilError runs the dispatch loop until either an opcode raises
// an error (returns finished=false, err≠nil) or OP_RET / OP_RET_NULL
// terminates the function (returns retVal, finished=true, err=nil).
func (f *Frame) runUntilError(ctx phpv.Context) (retVal *phpv.ZVal, finished bool, err error) {
	code := f.fn.Code
	for {
		ins := code[f.pc]
		f.pc++

		switch ins.Op() {
		case OpNop:

		case OpLoadConst:
			v := f.fn.CachedZ[ins.A()]
			f.push(v)

		case OpLoadNull:
			f.push(phpv.ZNULL.ZVal())
		case OpLoadTrue:
			f.push(phpv.ZTrue.ZVal())
		case OpLoadFalse:
			f.push(phpv.ZFalse.ZVal())

		case OpLoadLocal:
			v := f.locals[ins.A()]
			if v == nil {
				name := f.fn.Locals[ins.A()]
				// $this is special: throw an Error when accessed
				// outside an object context.
				if name == "this" {
					if ctx.This() == nil {
						return nil, false, phpobj.ThrowError(ctx, phpobj.Error, "Using $this when not in object context")
					}
					v = ctx.This().ZVal()
				} else {
					v = phpv.ZNULL.ZVal()
				}
			}
			f.push(v)

		case OpLoadLocalOrWarn:
			v := f.locals[ins.A()]
			if v == nil {
				name := f.fn.Locals[ins.A()]
				if name == "this" {
					if ctx.This() == nil {
						return nil, false, phpobj.ThrowError(ctx, phpobj.Error, "Using $this when not in object context")
					}
					v = ctx.This().ZVal()
				} else {
					if err := ctx.Warn("Undefined variable $%s", string(name), logopt.NoFuncName(true)); err != nil {
						return nil, false, err
					}
					v = phpv.ZNULL.ZVal()
				}
			}
			f.push(v)

		case OpStoreLocal:
			v := f.pop()
			if err := f.storeLocal(ctx, ins.A(), v); err != nil {
				return nil, false, err
			}

		case OpStoreLocalKeep:
			v := f.peek()
			if v != nil && v.IsCached() {
				// The peek'd value is also what's on top of the stack;
				// replace top with the duplicated copy so the consumer
				// sees the same canonical pointer that's now in the slot.
				dup := phpv.NewZVal(v.Value())
				f.replaceTop(dup)
				v = dup
			}
			if err := f.storeLocal(ctx, ins.A(), v); err != nil {
				return nil, false, err
			}

		case OpDup:
			f.push(f.peek())

		case OpPop:
			f.pop()

		// --- arithmetic / bitwise -----------------------------------
		// Int-int fast paths: when both operands are ZtInt we
		// compute inline and (on overflow) fall through to the
		// generic OperatorMath path which produces a float result.
		case OpAdd:
			if ai, bi, ok := f.intIntFast(); ok {
				c := int64(ai) + int64(bi)
				if (c > int64(ai)) == (bi > 0) {
					f.stack[f.sp-2] = phpv.ZInt(c).ZVal()
					f.sp--
					break
				}
			}
			if err := f.binop(ctx, opAdd); err != nil {
				return nil, false, err
			}
		case OpSub:
			if ai, bi, ok := f.intIntFast(); ok {
				c := int64(ai) - int64(bi)
				if (c < int64(ai)) == (bi > 0) {
					f.stack[f.sp-2] = phpv.ZInt(c).ZVal()
					f.sp--
					break
				}
			}
			if err := f.binop(ctx, opSub); err != nil {
				return nil, false, err
			}
		case OpMul:
			if ai, bi, ok := f.intIntFast(); ok {
				if ai == 0 || bi == 0 {
					f.stack[f.sp-2] = phpv.ZInt(0).ZVal()
					f.sp--
					break
				}
				c := int64(ai) * int64(bi)
				// Overflow check: c/b should equal a.
				if ((c < 0) == ((ai < 0) != (bi < 0))) && c/int64(bi) == int64(ai) {
					f.stack[f.sp-2] = phpv.ZInt(c).ZVal()
					f.sp--
					break
				}
			}
			if err := f.binop(ctx, opMul); err != nil {
				return nil, false, err
			}
		case OpDiv:
			if ai, bi, ok := f.intIntFast(); ok && bi != 0 {
				if !(ai == math.MinInt64 && bi == -1) && int64(ai)%int64(bi) == 0 {
					f.stack[f.sp-2] = phpv.ZInt(int64(ai) / int64(bi)).ZVal()
					f.sp--
					break
				}
			}
			if err := f.binop(ctx, opDiv); err != nil {
				return nil, false, err
			}
		case OpMod:
			if ai, bi, ok := f.intIntFast(); ok && bi != 0 && !(ai == math.MinInt64 && bi == -1) {
				f.stack[f.sp-2] = phpv.ZInt(int64(ai) % int64(bi)).ZVal()
				f.sp--
				break
			}
			if err := f.binop(ctx, opMod); err != nil {
				return nil, false, err
			}
		case OpPow:
			if err := f.binop(ctx, opPow); err != nil {
				return nil, false, err
			}
		case OpBitAnd:
			if ai, bi, ok := f.intIntFast(); ok {
				f.stack[f.sp-2] = phpv.ZInt(int64(ai) & int64(bi)).ZVal()
				f.sp--
				break
			}
			if err := f.binop(ctx, opBitAnd); err != nil {
				return nil, false, err
			}
		case OpBitOr:
			if ai, bi, ok := f.intIntFast(); ok {
				f.stack[f.sp-2] = phpv.ZInt(int64(ai) | int64(bi)).ZVal()
				f.sp--
				break
			}
			if err := f.binop(ctx, opBitOr); err != nil {
				return nil, false, err
			}
		case OpBitXor:
			if ai, bi, ok := f.intIntFast(); ok {
				f.stack[f.sp-2] = phpv.ZInt(int64(ai) ^ int64(bi)).ZVal()
				f.sp--
				break
			}
			if err := f.binop(ctx, opBitXor); err != nil {
				return nil, false, err
			}
		case OpShl:
			if ai, bi, ok := f.intIntFast(); ok && bi >= 0 {
				if bi >= 64 {
					f.stack[f.sp-2] = phpv.ZInt(0).ZVal()
				} else {
					f.stack[f.sp-2] = phpv.ZInt(int64(ai) << uint(bi)).ZVal()
				}
				f.sp--
				break
			}
			if err := f.binop(ctx, opShl); err != nil {
				return nil, false, err
			}
		case OpShr:
			if ai, bi, ok := f.intIntFast(); ok && bi >= 0 {
				if bi >= 64 {
					if ai < 0 {
						f.stack[f.sp-2] = phpv.ZInt(-1).ZVal()
					} else {
						f.stack[f.sp-2] = phpv.ZInt(0).ZVal()
					}
				} else {
					f.stack[f.sp-2] = phpv.ZInt(int64(ai) >> uint(bi)).ZVal()
				}
				f.sp--
				break
			}
			if err := f.binop(ctx, opShr); err != nil {
				return nil, false, err
			}
		case OpConcat:
			// String+string fast path: avoid OperatorAppend's full
			// type coercion + GMP overload check.
			b := f.stack[f.sp-1]
			a := f.stack[f.sp-2]
			if a.GetType() == phpv.ZtString && b.GetType() == phpv.ZtString {
				as := a.Value().(phpv.ZString)
				bs := b.Value().(phpv.ZString)
				f.stack[f.sp-2] = (as + bs).ZVal()
				f.sp--
				break
			}
			if err := f.binop(ctx, opConcat); err != nil {
				return nil, false, err
			}

		case OpNeg:
			if err := f.unop(ctx, opNeg); err != nil {
				return nil, false, err
			}
		case OpBitNot:
			if err := f.unop(ctx, opBitNot); err != nil {
				return nil, false, err
			}
		case OpNot:
			if err := f.unop(ctx, opNot); err != nil {
				return nil, false, err
			}

		// --- compare -------------------------------------------------
		// Int-int fast paths inline the comparison; the generic
		// path (OperatorCompare) only fires for mixed-type operands.
		case OpCmpEq:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai == bi)
				break
			}
			if err := f.binop(ctx, opCmpEq); err != nil {
				return nil, false, err
			}
		case OpCmpNe:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai != bi)
				break
			}
			if err := f.binop(ctx, opCmpNe); err != nil {
				return nil, false, err
			}
		case OpCmpId:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai == bi)
				break
			}
			if err := f.binop(ctx, opCmpId); err != nil {
				return nil, false, err
			}
		case OpCmpNid:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai != bi)
				break
			}
			if err := f.binop(ctx, opCmpNid); err != nil {
				return nil, false, err
			}
		case OpCmpLt:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai < bi)
				break
			}
			if err := f.binop(ctx, opCmpLt); err != nil {
				return nil, false, err
			}
		case OpCmpLe:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai <= bi)
				break
			}
			if err := f.binop(ctx, opCmpLe); err != nil {
				return nil, false, err
			}
		case OpCmpGt:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai > bi)
				break
			}
			if err := f.binop(ctx, opCmpGt); err != nil {
				return nil, false, err
			}
		case OpCmpGe:
			if ai, bi, ok := f.intIntFast(); ok {
				f.pushBoolReplace2(ai >= bi)
				break
			}
			if err := f.binop(ctx, opCmpGe); err != nil {
				return nil, false, err
			}
		case OpCmpSpaceship:
			if err := f.binop(ctx, opCmpSpaceship); err != nil {
				return nil, false, err
			}

		case OpBool:
			v := f.peek()
			f.replaceTop(phpv.ZBool(v.AsBool(ctx)).ZVal())

		// --- inc/dec on locals --------------------------------------
		// Int fast paths inline a slot-int + add + store without
		// going through DoInc's full type switch. Mixed types or
		// ref-bound slots fall through to the generic path so the
		// ref propagation in DoInc/storeLocal does its job.
		case OpIncLocal:
			cur := f.locals[ins.A()]
			if cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtInt {
				ci := int64(cur.Value().(phpv.ZInt))
				if ci != math.MaxInt64 {
					newV := phpv.ZInt(ci + 1).ZVal()
					f.locals[ins.A()] = newV
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
							return nil, false, err
						}
					}
					f.push(newV)
					break
				}
			}
			if err := f.incDecLocal(ctx, ins.A(), true, false); err != nil {
				return nil, false, err
			}
		case OpDecLocal:
			cur := f.locals[ins.A()]
			if cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtInt {
				ci := int64(cur.Value().(phpv.ZInt))
				if ci != math.MinInt64 {
					newV := phpv.ZInt(ci - 1).ZVal()
					f.locals[ins.A()] = newV
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
							return nil, false, err
						}
					}
					f.push(newV)
					break
				}
			}
			if err := f.incDecLocal(ctx, ins.A(), false, false); err != nil {
				return nil, false, err
			}
		case OpPostIncLocal:
			cur := f.locals[ins.A()]
			if cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtInt {
				ci := int64(cur.Value().(phpv.ZInt))
				if ci != math.MaxInt64 {
					newV := phpv.ZInt(ci + 1).ZVal()
					f.locals[ins.A()] = newV
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
							return nil, false, err
						}
					}
					f.push(cur)
					break
				}
			}
			if err := f.incDecLocal(ctx, ins.A(), true, true); err != nil {
				return nil, false, err
			}
		case OpPostDecLocal:
			cur := f.locals[ins.A()]
			if cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtInt {
				ci := int64(cur.Value().(phpv.ZInt))
				if ci != math.MinInt64 {
					newV := phpv.ZInt(ci - 1).ZVal()
					f.locals[ins.A()] = newV
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
							return nil, false, err
						}
					}
					f.push(cur)
					break
				}
			}
			if err := f.incDecLocal(ctx, ins.A(), false, true); err != nil {
				return nil, false, err
			}

		// --- compound assign on locals ------------------------------
		case OpOpAssignLocal:
			op := tokenizer.ItemType(ins.B())
			rhs := f.stack[f.sp-1]
			cur := f.locals[ins.A()]
			// Int += int fast path — common in arithmetic loops.
			if op == tokenizer.T_PLUS_EQUAL &&
				cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtInt && rhs.GetType() == phpv.ZtInt {
				ai := int64(cur.Value().(phpv.ZInt))
				bi := int64(rhs.Value().(phpv.ZInt))
				c := ai + bi
				if (c > ai) == (bi > 0) {
					newV := phpv.ZInt(c).ZVal()
					f.locals[ins.A()] = newV
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
							return nil, false, err
						}
					}
					f.sp--
					break
				}
			}
			// String .= string fast path — common in string-build
			// loops.
			if op == tokenizer.T_CONCAT_EQUAL &&
				cur != nil && !cur.IsRef() && cur.GetType() == phpv.ZtString && rhs.GetType() == phpv.ZtString {
				as := cur.Value().(phpv.ZString)
				bs := rhs.Value().(phpv.ZString)
				newV := (as + bs).ZVal()
				f.locals[ins.A()] = newV
				if !f.fn.SlotOnly {
					if err := ctx.OffsetSet(ctx, f.fn.Locals[ins.A()], newV); err != nil {
						return nil, false, err
					}
				}
				f.sp--
				break
			}
			f.sp-- // pop rhs (was peek)
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: unknown compound op %d", ins.B())
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			// Snapshot LHS before the operator runs: `.=` triggers
			// array→string coercion which may fire __toString or a
			// user-installed error_handler that mutates the same slot
			// (bug81705). With a ref-captured slot, the handler would
			// otherwise mutate `cur` in-place and the operator would
			// read the post-handler value instead of the pre-handler
			// one PHP requires.
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			if err := f.storeLocal(ctx, ins.A(), res); err != nil {
				return nil, false, err
			}

		// --- control flow -------------------------------------------
		case OpJmp:
			f.pc = uint32(int32(f.pc) + ins.C())

		case OpJmpIfFalse:
			v := f.pop()
			// Bool fast path — comparison ops always produce ZBool,
			// so the common case (loop condition `$i < N`, if
			// expressions, etc.) never goes through the generic
			// AsBool → As → interface conversion chain.
			if zb, ok := v.Value().(phpv.ZBool); ok {
				if !bool(zb) {
					f.pc = uint32(int32(f.pc) + ins.C())
				}
			} else if !bool(v.AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfTrue:
			v := f.pop()
			if zb, ok := v.Value().(phpv.ZBool); ok {
				if bool(zb) {
					f.pc = uint32(int32(f.pc) + ins.C())
				}
			} else if bool(v.AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfFalsePeek:
			v := f.peek()
			if zb, ok := v.Value().(phpv.ZBool); ok {
				if !bool(zb) {
					f.pc = uint32(int32(f.pc) + ins.C())
				}
			} else if !bool(v.AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfTruePeek:
			if bool(f.peek().AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfNotNullPeek:
			if f.peek().GetType() != phpv.ZtNull {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		// --- call ----------------------------------------------------
		case OpCallUser:
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_CALL_USER name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			argc := int(ins.B())
			if err := f.callUser(ctx, name, argc, ins.A()); err != nil {
				return nil, false, err
			}
			// callUser pops argc and pushes 1 result.

		case OpCallIndirect:
			argc := int(ins.B())
			args := make([]*phpv.ZVal, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = f.pop()
			}
			callable := f.pop()
			c, this, err := compiler.ResolveCallable(ctx, callable)
			if err != nil {
				return nil, false, err
			}
			var res *phpv.ZVal
			if this != nil {
				res, err = ctx.CallZVal(ctx, c, args, this)
			} else {
				res, err = ctx.CallZVal(ctx, c, args, nil)
			}
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpCallUserByExprs:
			// By-ref / named / spread variant. A = const-pool name idx,
			// B = SubArgs index of the argument-expression list. The
			// callable is resolved at runtime (with CallableCache reuse)
			// then handed to ctx.Call which evaluates each expression
			// with full binding semantics.
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_CALL_USER_BY_EXPRS name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			nameIdx := ins.A()
			var callable phpv.Callable
			if int(nameIdx) < len(f.fn.CallableCache) {
				callable = f.fn.CallableCache[nameIdx]
			}
			if callable == nil {
				c, err := ctx.Global().GetFunction(ctx, name)
				if err != nil {
					return nil, false, err
				}
				callable = c
				if f.fn.CallableCache == nil {
					f.fn.CallableCache = make([]phpv.Callable, len(f.fn.Consts))
				}
				if int(nameIdx) < len(f.fn.CallableCache) {
					f.fn.CallableCache[nameIdx] = callable
				}
			}
			argsIdx := int(ins.B())
			if argsIdx >= len(f.fn.SubArgs) {
				return nil, false, fmt.Errorf("vm: OP_CALL_USER_BY_EXPRS SubArgs index %d out of range", argsIdx)
			}
			res, err := ctx.Call(ctx, callable, f.fn.SubArgs[argsIdx], nil)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpCallIndirectByExprs:
			// A = SubArgs index. Callable expression already on top of
			// stack. Pop it, resolve via ResolveCallable, then dispatch
			// via ctx.Call so by-ref / named / spread args bind through
			// the standard call-binding pipeline.
			callable := f.pop()
			c, this, err := compiler.ResolveCallable(ctx, callable)
			if err != nil {
				return nil, false, err
			}
			argsIdx := int(ins.A())
			if argsIdx >= len(f.fn.SubArgs) {
				return nil, false, fmt.Errorf("vm: OP_CALL_INDIRECT_BY_EXPRS SubArgs index %d out of range", argsIdx)
			}
			exprs := f.fn.SubArgs[argsIdx]
			var res *phpv.ZVal
			if this != nil {
				res, err = ctx.Call(ctx, c, exprs, this)
			} else {
				res, err = ctx.Call(ctx, c, exprs, nil)
			}
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		// --- return --------------------------------------------------
		case OpRet:
			v := f.pop()
			// If an enclosing try-with-finally protects this PC, defer
			// the return: park the value in that handler's pending slot
			// and jump to its finally body. OP_FINALLY_END for that
			// handler will either chain to an outer finally or perform
			// the actual return.
			if hidx, h, ok := f.findEnclosingFinally(f.pc - 1); ok {
				f.handlerPending[hidx] = pendingControl{kind: pendingReturn, val: v}
				f.resetStackTo(h.StackBase)
				f.pc = h.FinallyPC
				continue
			}
			return v, true, nil

		case OpRetNull:
			if hidx, h, ok := f.findEnclosingFinally(f.pc - 1); ok {
				f.handlerPending[hidx] = pendingControl{kind: pendingReturn, val: phpv.ZNULL.ZVal()}
				f.resetStackTo(h.StackBase)
				f.pc = h.FinallyPC
				continue
			}
			return phpv.ZNULL.ZVal(), true, nil

		case OpFinallyEnd:
			// A=handler index. Read this finally body's pending slot,
			// then clear it so a re-entry (e.g. via a loop containing
			// the try) starts fresh.
			hidx := int(ins.A())
			p := f.handlerPending[hidx]
			f.handlerPending[hidx] = pendingControl{}
			switch p.kind {
			case pendingNone:
				// Normal completion of try (or try+catch). Fall through
				// to whatever comes after the finally body.
			case pendingReturn:
				// Chain into an outer finally if one covers the
				// post-finally PC; otherwise perform the actual return.
				if oidx, oh, ok := f.findEnclosingFinally(f.pc - 1); ok {
					f.handlerPending[oidx] = pendingControl{kind: pendingReturn, val: p.val}
					f.resetStackTo(oh.StackBase)
					f.pc = oh.FinallyPC
					continue
				}
				return p.val, true, nil
			case pendingThrow:
				// Re-raise the held throw. exec's dispatch loop will
				// re-enter dispatchTryHandler with failPC=current pc,
				// finding an outer catch / outer finally as appropriate.
				return nil, false, p.err
			}

		// --- arrays --------------------------------------------------
		case OpNewArray:
			arr := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
			f.push(arr.ZVal())

		case OpArrayInitAppend:
			val := f.pop()
			// Snapshot array values into the literal — PHP array
			// literals copy by value.
			if val != nil && val.GetType() == phpv.ZtArray && !val.IsRef() {
				val = val.Dup()
			}
			arr := f.peek().AsArray(ctx)
			if err := arr.OffsetSet(ctx, nil, val); err != nil {
				if err == phpv.ErrNextElementOccupied {
					return nil, false, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
				}
				return nil, false, err
			}

		case OpArrayInitKeyed:
			val := f.pop()
			key := f.pop()
			if val != nil && val.GetType() == phpv.ZtArray && !val.IsRef() {
				val = val.Dup()
			}
			// Mirror runArray.Run key-type validation: forbid
			// object/array keys, deprecate null keys, emit float-
			// precision deprecation, and cast resource keys to int.
			switch key.GetType() {
			case phpv.ZtObject:
				return nil, false, phpobj.ThrowError(ctx, phpobj.TypeError,
					"Cannot access offset of type object on array")
			case phpv.ZtArray:
				return nil, false, phpobj.ThrowError(ctx, phpobj.TypeError,
					"Cannot access offset of type array on array")
			case phpv.ZtFloat:
				if _, err := phpv.FloatToIntImplicit(ctx, key.Value().(phpv.ZFloat)); err != nil {
					return nil, false, err
				}
			case phpv.ZtNull:
				if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
					return nil, false, err
				}
			case phpv.ZtResource:
				if r, ok := key.Value().(phpv.Resource); ok {
					id := r.GetResourceID()
					if err := ctx.Warn("Resource ID#%d used as offset, casting to integer (%d)", id, id); err != nil {
						return nil, false, err
					}
					key = phpv.ZInt(id).ZVal()
				}
			}
			arr := f.peek().AsArray(ctx)
			if err := arr.OffsetSet(ctx, key.Value(), val); err != nil {
				if err == phpv.ErrNextElementOccupied {
					return nil, false, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
				}
				return nil, false, err
			}

		case OpArrayGet:
			offset := f.pop()
			container := f.pop()
			res, err := arrayGet(ctx, container, offset)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpArraySetLocal:
			val := f.pop()
			offset := f.pop()
			if err := arraySetLocal(ctx, f, ins.A(), offset, val); err != nil {
				return nil, false, err
			}

		case OpArrayAppendLocal:
			val := f.pop()
			if err := arraySetLocal(ctx, f, ins.A(), nil, val); err != nil {
				return nil, false, err
			}

		case OpArrayPreCheckLocal:
			// `$local[expr] OP= …` / `$local[expr]++` pre-check, runs
			// BEFORE the offset is evaluated so warnings about the
			// container fire in the right order relative to the offset's
			// own evaluation warnings.
			//
			// Mirrors the AST's compound-write-context handling in
			// runArrayAccess.Run (compile-array.go:486-528): undefined
			// variables warn and auto-vivify, ZtNull auto-vivifies, and
			// string containers reject compound ops with the same
			// "Cannot use assign-op operators with string offsets" Error
			// the AST throws before touching the offset (bug53432).
			{
				name := f.fn.Locals[ins.A()]
				container := f.locals[ins.A()]
				wasUndefined := container == nil
				if container == nil && !f.fn.SlotOnly {
					if v, found, _ := ctx.OffsetCheck(ctx, name); found && v != nil {
						container = v
						wasUndefined = false
					}
				}
				if container != nil && container.GetType() == phpv.ZtString {
					return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot use assign-op operators with string offsets")
				}
				if container == nil || container.GetType() == phpv.ZtNull {
					if wasUndefined {
						if err := ctx.Warn("Undefined variable $%s", string(name), logopt.NoFuncName(true)); err != nil {
							return nil, false, err
						}
					}
					newArr := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
					container = newArr.ZVal()
					f.locals[ins.A()] = container
					if !f.fn.SlotOnly {
						if err := ctx.OffsetSet(ctx, name, container); err != nil {
							return nil, false, err
						}
					}
				}
			}

		case OpArrayCompoundAssignLocal:
			// `$local[offset] OP= rhs` on a simple-local container.
			// Mirrors OpObjectCompoundAssign's read+snapshot+op+write
			// flow but with the array element as the LHS slot.
			//
			// The container is assumed to already be array-shaped (or a
			// scalar that arrayGet/arraySetLocal know how to reject) —
			// OP_ARRAY_PRE_CHECK_LOCAL emitted earlier has vivified null
			// containers and rejected string ones.
			rhs := f.pop()
			offset := f.pop()
			op := tokenizer.ItemType(ins.B())
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: OP_ARRAY_COMPOUND_ASSIGN_LOCAL unknown op %d", ins.B())
			}
			container := f.locals[ins.A()]
			if container == nil {
				container = phpv.ZNULL.ZVal()
			}
			// PHP 8.1: deprecate null-as-offset in the read phase for
			// array/string/object containers, matching the AST
			// (compile-array.go:712-717). arraySetLocal does NOT also
			// emit this on the write phase (would double-warn).
			if offset != nil && offset.GetType() == phpv.ZtNull {
				ct := container.GetType()
				if ct == phpv.ZtArray || ct == phpv.ZtString || ct == phpv.ZtObject {
					if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
						return nil, false, err
					}
				}
			}
			// bug70662: track whether the key existed BEFORE arrayGet.
			// If arrayGet's "Undefined array key" warning triggers a
			// user error handler that creates the key, PHP's compound
			// op suppresses the write (the handler's value wins).
			var keyWasMissing bool
			if container.GetType() == phpv.ZtArray && offset != nil {
				zarr := container.AsArray(ctx)
				_, exists, _ := zarr.OffsetCheck(ctx, offset.Value())
				keyWasMissing = !exists
			}
			cur, err := arrayGet(ctx, container, offset)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			// Snapshot cur — `.=` on an array element can trigger
			// __toString / error_handler side effects that mutate the
			// underlying slot mid-op (bug81705-family). Without the
			// Dup, the operator would read the post-handler value.
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			// Re-fetch container — an error handler triggered during
			// arrayGet (undef-key) or fn (__toString) could have
			// replaced the slot with something incompatible.
			postContainer := f.locals[ins.A()]
			if postContainer == nil || (postContainer.GetType() != phpv.ZtArray &&
				postContainer.GetType() != phpv.ZtObject &&
				postContainer.GetType() != phpv.ZtString &&
				postContainer.GetType() != phpv.ZtNull) {
				// Container was replaced by a scalar — abort write to
				// match AST's WriteValue behavior (compile-array.go:826-832).
				if ins.C()&1 != 0 {
					f.push(res)
				}
				break
			}
			// bug70662: if the key was missing during the read phase but
			// now exists in the container, an error handler created it.
			// Suppress the write so the handler's value remains visible.
			if keyWasMissing && offset != nil && postContainer.GetType() == phpv.ZtArray {
				zarr := postContainer.AsArray(ctx)
				if _, exists, _ := zarr.OffsetCheck(ctx, offset.Value()); exists {
					if ins.C()&1 != 0 {
						f.push(res)
					}
					break
				}
			}
			if err := arraySetLocal(ctx, f, ins.A(), offset, res); err != nil {
				return nil, false, err
			}
			if ins.C()&1 != 0 {
				f.push(res)
			}

		case OpArrayIncDecLocal:
			// `$local[offset]++`, `++$local[offset]` and dec variants
			// on a simple-local container. Reads via arrayGet (which
			// warns on undefined keys, matching the AST), Dup's so
			// DoInc's in-place mutation doesn't escape, applies DoInc,
			// writes back via arraySetLocal.
			//
			// OP_ARRAY_PRE_CHECK_LOCAL emitted earlier vivifies null
			// containers and rejects string ones.
			offset := f.pop()
			inc := ins.B()&1 != 0
			post := ins.B()&2 != 0
			keep := ins.C()&1 != 0
			container := f.locals[ins.A()]
			if container == nil {
				container = phpv.ZNULL.ZVal()
			}
			// PHP 8.1 null-offset deprecation (same as compound path).
			if offset != nil && offset.GetType() == phpv.ZtNull {
				ct := container.GetType()
				if ct == phpv.ZtArray || ct == phpv.ZtString || ct == phpv.ZtObject {
					if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
						return nil, false, err
					}
				}
			}
			var keyWasMissing bool
			if container.GetType() == phpv.ZtArray && offset != nil {
				zarr := container.AsArray(ctx)
				_, exists, _ := zarr.OffsetCheck(ctx, offset.Value())
				keyWasMissing = !exists
			}
			cur, err := arrayGet(ctx, container, offset)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			var pre *phpv.ZVal
			if post {
				pre = cur.Dup()
			}
			if err := compiler.DoInc(ctx, cur, inc); err != nil {
				return nil, false, err
			}
			postContainer := f.locals[ins.A()]
			if postContainer == nil || (postContainer.GetType() != phpv.ZtArray &&
				postContainer.GetType() != phpv.ZtObject &&
				postContainer.GetType() != phpv.ZtString &&
				postContainer.GetType() != phpv.ZtNull) {
				if keep {
					if post {
						f.push(pre)
					} else {
						f.push(cur)
					}
				}
				break
			}
			if keyWasMissing && offset != nil && postContainer.GetType() == phpv.ZtArray {
				zarr := postContainer.AsArray(ctx)
				if _, exists, _ := zarr.OffsetCheck(ctx, offset.Value()); exists {
					if keep {
						if post {
							f.push(pre)
						} else {
							f.push(cur)
						}
					}
					break
				}
			}
			if err := arraySetLocal(ctx, f, ins.A(), offset, cur); err != nil {
				return nil, false, err
			}
			if keep {
				if post {
					f.push(pre)
				} else {
					f.push(cur)
				}
			}

		// --- throw ---------------------------------------------------
		case OpThrow:
			v := f.pop()
			return nil, false, phpobj.ThrowObject(ctx, v)

		// --- closures ------------------------------------------------
		case OpMakeClosure:
			r := f.fn.SubClosures[ins.A()]
			res, err := r.Run(ctx)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpClassConst:
			r := f.fn.SubASTs[ins.A()]
			res, err := r.Run(ctx)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpTryFinally:
			r := f.fn.SubASTs[ins.A()]
			if _, err := r.Run(ctx); err != nil {
				return nil, false, err
			}
			// AST delegation may have written locals through the
			// FuncContext hashtable. Resync the slot cache so
			// subsequent OP_LOAD_LOCAL reads see the new values.
			f.refreshSlots(ctx)

		case OpRefreshSlots:
			f.refreshSlots(ctx)

		// --- objects -------------------------------------------------
		case OpNewObject:
			argc := int(ins.B())
			args := make([]*phpv.ZVal, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = f.pop()
			}
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_NEW_OBJECT name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			class, err := ctx.Global().GetClass(ctx, name, true)
			if err != nil {
				return nil, false, err
			}
			obj, err := phpobj.NewZObject(ctx, class, args...)
			if err != nil {
				return nil, false, err
			}
			f.push(obj.ZVal())

		case OpObjectGet:
			receiver := f.pop()
			// B != 0 marks a nullsafe read (`$obj?->prop`): short-
			// circuit to null when the receiver itself is null.
			if ins.B() != 0 && (receiver == nil || phpv.IsNull(receiver)) {
				f.push(phpv.ZNULL.ZVal())
				break
			}
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_GET name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			res, err := objectGet(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpObjectSet:
			val := f.pop()
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_SET name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			if err := objectSet(ctx, receiver, name, val); err != nil {
				return nil, false, err
			}
			// B != 0: expression-context, leave value on stack.
			if ins.B() != 0 {
				f.push(val)
			}

		case OpObjectGetSafe:
			// Permissive read for coalesce LHS: silences "Undefined
			// property" warnings on object receivers. Non-object
			// receiver warnings (Attempt to read property on null, etc.)
			// still fire — matches PHP's `??` semantics.
			receiver := f.pop()
			if ins.B() != 0 && (receiver == nil || phpv.IsNull(receiver)) {
				f.push(phpv.ZNULL.ZVal())
				break
			}
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_GET_SAFE name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			res, err := objectGetSafe(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpIssetObjProp:
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_ISSET_OBJ_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			exists, err := compiler.EvalIssetObjProp(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(exists).ZVal())

		case OpEmptyObjProp:
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_EMPTY_OBJ_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			isEmpty, err := compiler.EvalEmptyObjProp(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(isEmpty).ZVal())

		case OpUnsetObjProp:
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_UNSET_OBJ_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			if err := compiler.EvalUnsetObjProp(ctx, receiver, name); err != nil {
				return nil, false, err
			}

		case OpObjectCompoundAssign:
			rhs := f.pop()
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_COMPOUND_ASSIGN name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			op := tokenizer.ItemType(ins.B())
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_COMPOUND_ASSIGN unknown op %d", ins.B())
			}
			cur, err := objectGet(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			// Snapshot cur — mirrors OpOpAssignLocal's defense
			// against handlers that mutate the slot in-place during
			// `.=` coercion (bug81705 family).
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			if err := objectSet(ctx, receiver, name, res); err != nil {
				return nil, false, err
			}
			if ins.C()&1 != 0 {
				f.push(res)
			}

		case OpObjectCall:
			argc := int(ins.B())
			args := make([]*phpv.ZVal, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = f.pop()
			}
			receiver := f.pop()
			// C != 0 marks a nullsafe call (`$obj?->m(...)`): short-
			// circuit to null when the receiver itself is null,
			// without evaluating the method dispatch.
			if ins.C() != 0 && (receiver == nil || phpv.IsNull(receiver)) {
				f.push(phpv.ZNULL.ZVal())
				break
			}
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_CALL name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			res, err := objectCall(ctx, receiver, name, args)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpObjectCallByExprs:
			// By-ref / named / spread variant. A = const-pool method
			// name idx, B = SubArgs index, C != 0 marks nullsafe. The
			// receiver is on top of stack; the call dispatch goes
			// through CallInstanceMethodByExprs which mirrors
			// CallInstanceMethod but uses ctx.Call so by-ref params
			// bind correctly.
			receiver := f.pop()
			if ins.C() != 0 && (receiver == nil || phpv.IsNull(receiver)) {
				f.push(phpv.ZNULL.ZVal())
				break
			}
			if receiver == nil || receiver.GetType() != phpv.ZtObject {
				typeName := "null"
				if receiver != nil {
					typeName = receiver.GetType().TypeName()
				}
				name, _ := f.fn.Consts[ins.A()].(phpv.ZString)
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to a member function %s() on %s", string(name), typeName))
			}
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_CALL_BY_EXPRS name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			argsIdx := int(ins.B())
			if argsIdx >= len(f.fn.SubArgs) {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_CALL_BY_EXPRS SubArgs index %d out of range", argsIdx)
			}
			res, err := compiler.CallInstanceMethodByExprs(ctx, receiver, name, f.fn.SubArgs[argsIdx])
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		// --- foreach -------------------------------------------------
		case OpForeachInit:
			src := f.pop()
			it, err := foreachInit(ctx, src)
			if err != nil {
				return nil, false, err
			}
			if it == nil {
				// Not iterable: emit warning and jump past the loop.
				if err := foreachWarnInvalid(ctx, src, f.fn.LocAt(f.pc-1)); err != nil {
					return nil, false, err
				}
				f.pc = uint32(int32(f.pc) + ins.C())
				break
			}
			f.iters = append(f.iters, it)

		case OpForeachStep:
			it := f.iters[len(f.iters)-1]
			if !it.Valid(ctx) {
				// Iterator exhausted — jump to the unwind target,
				// which pops the iterator. We don't pop here so that
				// OpForeachUnwind sees the same iterator regardless
				// of whether we got there via natural end or `break`.
				f.pc = uint32(int32(f.pc) + ins.C())
				break
			}
			cur, err := it.Current(ctx)
			if err != nil {
				return nil, false, err
			}
			if err := f.storeLocal(ctx, ins.A(), cur); err != nil {
				return nil, false, err
			}
			if ins.B() != 0xFFFF {
				key, err := it.Key(ctx)
				if err != nil {
					return nil, false, err
				}
				if err := f.storeLocal(ctx, ins.B(), key); err != nil {
					return nil, false, err
				}
			}

		case OpForeachAdvance:
			it := f.iters[len(f.iters)-1]
			if _, err := it.Next(ctx); err != nil {
				return nil, false, err
			}

		case OpForeachStepPush:
			it := f.iters[len(f.iters)-1]
			if !it.Valid(ctx) {
				f.pc = uint32(int32(f.pc) + ins.C())
				break
			}
			cur, err := it.Current(ctx)
			if err != nil {
				return nil, false, err
			}
			// Match AST runner's "key written before value" order:
			// push key first (deeper in stack), then value (on top).
			// Consumers pop value first, assign value; then pop key,
			// assign key — which yields key-write-then-value-write at
			// the WriteValue level.
			if ins.A() != 0 {
				key, err := it.Key(ctx)
				if err != nil {
					return nil, false, err
				}
				f.push(key.Dup())
			}
			f.push(cur.Dup())

		case OpAssignWritable:
			val := f.pop()
			lhs, ok := f.fn.SubASTs[ins.A()].(phpv.Writable)
			if !ok {
				return nil, false, fmt.Errorf("OP_ASSIGN_WRITABLE: SubASTs[%d] is not Writable (%T)", ins.A(), f.fn.SubASTs[ins.A()])
			}
			if err := lhs.WriteValue(ctx, val); err != nil {
				return nil, false, err
			}

		case OpForeachUnwind:
			it := f.iters[len(f.iters)-1]
			if c, ok := it.(interface{ Cleanup() }); ok {
				c.Cleanup()
			}
			f.iters = f.iters[:len(f.iters)-1]

		// --- diagnostics --------------------------------------------
		case OpTick:
			if err := ctx.Tick(ctx, f.fn.LocAt(f.pc-1)); err != nil {
				return nil, false, err
			}
			ctx.Global().DrainTempObjects()

		// --- instanceof ---------------------------------------------
		case OpInstanceOf:
			clsVal := f.pop()
			v := f.pop()
			var className phpv.ZString
			if clsVal.GetType() == phpv.ZtObject {
				className = clsVal.Value().(phpv.ZObject).GetClass().GetName()
			} else {
				className = clsVal.AsString(ctx)
			}
			res, err := compiler.EvalInstanceOf(ctx, v, className)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- clone --------------------------------------------------
		case OpClone:
			v := f.pop()
			res, err := compiler.EvalClone(ctx, v)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- class-name-of (Cls::class / $obj::class) ---------------
		case OpClassNameOf:
			v := f.pop()
			isLiteral := ins.A() != 0
			res, err := compiler.EvalClassNameOf(ctx, v, isLiteral, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- inline HTML --------------------------------------------
		case OpInlineHtml:
			s, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_INLINE_HTML const is %T not ZString", f.fn.Consts[ins.A()])
			}
			if _, err := ctx.Write([]byte(s)); err != nil {
				return nil, false, err
			}

		// --- declare(strict_types=1) --------------------------------
		case OpSetStrictTypes:
			ctx.Global().SetStrictTypes(true)

		// --- first-class callable (func(...)) -----------------------
		case OpFirstClassCallable:
			v := f.pop()
			res, err := compiler.ClosureFromCallable(ctx, v)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- first-class clone (clone(...)) -------------------------
		case OpFirstClassClone:
			res, err := compiler.EvalFirstClassCloneCallable(ctx)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- method first-class ($obj->m(...) / Cls::m(...)) -------
		case OpMethodFirstClass:
			recv := f.pop()
			method, ok := f.fn.Consts[ins.B()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_METHOD_FIRSTCLASS name const is %T not ZString", f.fn.Consts[ins.B()])
			}
			static := ins.A()&1 != 0
			nullsafe := ins.A()&2 != 0
			res, err := compiler.EvalMethodFirstClassCallable(ctx, recv, method, static, nullsafe)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- dyn-method first-class ($obj->{expr}(...)) -------------
		case OpDynMethodFirstClass:
			nameV := f.pop()
			recv := f.pop()
			res, err := compiler.EvalDynMethodFirstClassCallable(ctx, recv, nameV)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- $obj->{$name} / $obj?->{$name} read --------------------
		case OpObjectDynGet:
			nameV := f.pop()
			recv := f.pop()
			// Bit 0 of A is the nullsafe flag: short-circuit to null
			// when the receiver itself is null.
			if ins.A()&1 != 0 && (recv == nil || phpv.IsNull(recv)) {
				f.push(phpv.ZNULL.ZVal())
			} else {
				res, err := compiler.EvalObjectDynVarRead(ctx, recv, nameV)
				if err != nil {
					return nil, false, err
				}
				f.push(res)
			}

		// --- Cls::${$x} = v / Cls::$$x = v static-prop dyn write -----
		case OpStaticPropDynSet:
			val := f.pop()
			nameV := f.pop()
			classV := f.pop()
			varName := phpv.ZString(nameV.AsString(ctx))
			if err := compiler.AssignClassStaticDynProp(ctx, classV, varName, val); err != nil {
				return nil, false, err
			}
			if ins.A()&1 != 0 {
				f.push(val)
			}

		// --- $obj->$x = v / $obj->{$x} = v dyn-name write ------------
		case OpObjectDynSet:
			val := f.pop()
			nameV := f.pop()
			recv := f.pop()
			if recv == nil || recv.Value() == nil {
				propName := ""
				if nameV != nil {
					propName = string(nameV.AsString(ctx))
				}
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to assign property \"%s\" on null", propName))
			}
			objI, ok := recv.Value().(phpv.ZObjectAccess)
			if !ok {
				typeName := compiler.PhpValueTypeName(recv)
				propName := ""
				if nameV != nil {
					propName = string(nameV.AsString(ctx))
				}
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to assign property \"%s\" on %s", propName, typeName))
			}
			if err := objI.ObjectSet(ctx, nameV, val); err != nil {
				return nil, false, err
			}
			// A bit 0: keep value on stack (expr context).
			if ins.A()&1 != 0 {
				f.push(val)
			}

		// --- global $x ---------------------------------------------
		case OpGlobalBind:
			nameV := f.pop()
			if err := compiler.EvalGlobalBinding(ctx, nameV.AsString(ctx)); err != nil {
				return nil, false, err
			}

		// --- class const fetch -------------------------------------
		case OpClassDynConst:
			nameV := f.pop()
			classV := f.pop()
			res, err := compiler.EvalClassDynConst(ctx, classV, nameV)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- Cls::$prop static-prop read ----------------------------
		case OpClassStaticGet:
			classV := f.pop()
			varName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_CLASS_STATIC_GET name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			res, err := compiler.EvalClassStaticVarRead(ctx, classV, varName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- Cls::IDENT class const / enum case fetch ---------------
		case OpClassStaticObjRef:
			classV := f.pop()
			objName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_CLASS_STATIC_OBJREF name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			res, err := compiler.EvalClassStaticObjRef(ctx, classV, objName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- Cls::${$name} dyn-name static-prop read ---------------
		case OpClassStaticDynGet:
			nameV := f.pop()
			classV := f.pop()
			varName := phpv.ZString(nameV.String())
			res, err := compiler.EvalClassStaticDynVarRead(ctx, classV, varName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- declare(ticks=N) tick callout --------------------------
		case OpCallTickFunctions:
			if err := ctx.Global().CallTickFunctions(ctx); err != nil {
				return nil, false, err
			}

		// --- `isset($localVar)` ------------------------------------
		case OpIssetLocal:
			v := f.locals[ins.A()]
			set := v != nil && !phpv.IsNull(v)
			f.push(phpv.ZBool(set).ZVal())

		// --- `empty($localVar)` ------------------------------------
		case OpEmptyLocal:
			v := f.locals[ins.A()]
			empty := compiler.IsValueEmpty(ctx, v)
			f.push(phpv.ZBool(empty).ZVal())

		// --- `match` no-arm-matched fall-through --------------------
		case OpThrowUnhandledMatch:
			cond := f.pop()
			return nil, false, compiler.ThrowUnhandledMatch(ctx, cond)

		// --- enum declaration registration --------------------------
		case OpRegisterEnum:
			node := f.fn.SubASTs[ins.A()]
			if err := compiler.RegisterEnum(ctx, node); err != nil {
				return nil, false, err
			}

		// --- `#[NoDiscard]`-wrapped statement bracket ---------------
		// The prev flag lives only in the slot — never mirrored to the
		// FuncContext hashtable since the synthetic local name
		// `__nodiscard_prev_N` shouldn't be visible to user code (and
		// the hashtable mirror was contributing to a Tick/Loc recursion
		// observed when the panic-recovery path on bug21478 left a
		// FuncContext in a partially-released state).
		case OpNoDiscardEnter:
			prev := compiler.NoDiscardEnter(ctx)
			f.locals[ins.A()] = phpv.ZBool(prev).ZVal()
		case OpNoDiscardExit:
			prev := false
			if v := f.locals[ins.A()]; v != nil {
				prev = bool(v.AsBool(ctx))
			}
			if err := compiler.NoDiscardExit(ctx, prev); err != nil {
				return nil, false, err
			}

		// --- `[…, …] = $rhs` / `list(…) = $rhs` destructure --------
		case OpDestructureAssign:
			rhs := f.pop()
			lhs := f.fn.SubASTs[ins.A()]
			if err := compiler.AssignDestructure(ctx, lhs, rhs); err != nil {
				return nil, false, err
			}
			if ins.B()&1 != 0 {
				f.push(rhs)
			}

		// --- `static $x = init; …` declaration --------------------
		case OpStaticVarBind:
			node := f.fn.SubASTs[ins.A()]
			if err := compiler.BindStaticVars(ctx, node); err != nil {
				return nil, false, err
			}

		// --- isset-permissive nested chain read ---------------------
		case OpArrayGetSafe:
			key := f.pop()
			cont := f.pop()
			res := compiler.IssetChainElement(ctx, cont, key)
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		// --- `isset($container[$key])` -----------------------------
		case OpIssetDim:
			key := f.pop()
			cont := f.pop()
			res, err := compiler.EvalIssetDim(ctx, cont, key)
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(res).ZVal())

		// --- `empty($container[$key])` ------------------------------
		case OpEmptyDim:
			key := f.pop()
			cont := f.pop()
			res, err := compiler.EvalEmptyDim(ctx, cont, key)
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(res).ZVal())

		// --- `unset($container[$key])` ------------------------------
		case OpUnsetDim:
			key := f.pop()
			cont := f.pop()
			if err := compiler.UnsetArrayDim(ctx, cont, key); err != nil {
				return nil, false, err
			}

		// --- `Foo::method(args)` static-style method call -----------
		case OpStaticMethodCall:
			node := f.fn.SubASTs[ins.A()]
			res, err := compiler.EvalStaticMethodCall(ctx, node)
			if err != nil {
				return nil, false, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		// --- extended `clone(…)` PHP 8.5+ forms ---------------------
		case OpCloneExt:
			node := f.fn.SubASTs[ins.A()]
			res, err := compiler.EvalCloneExt(ctx, node)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `&$expr` reference creation ----------------------------
		case OpCreateRef:
			node := f.fn.SubASTs[ins.A()]
			res, err := compiler.EvalCreateRef(ctx, node)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `$obj->{$expr}(args)` dyn-method-name call ------------
		case OpObjectDynCall:
			// Stack: [..., receiver, method-name]. Args are looked up
			// from the AST node (SubASTs[A]); ctx.Call in the helper
			// applies by-ref / named / spread binding.
			node := f.fn.SubASTs[ins.A()]
			name := f.pop()
			recv := f.pop()
			res, err := compiler.EvalObjectDynFuncWithEvaluated(ctx, node, recv, name)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `Foo::$bar = v` static property write ------------------
		case OpStaticPropSet:
			val := f.pop()
			classV := f.pop()
			varName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_STATIC_PROP_SET name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			if err := compiler.AssignClassStaticProp(ctx, classV, varName, val); err != nil {
				return nil, false, err
			}
			// B != 0: expression-context, leave value on stack.
			if ins.B() != 0 {
				f.push(val)
			}

		// --- `isset(Cls::$prop)` / `empty(Cls::$prop)` ----------------
		case OpIssetStaticProp:
			classV := f.pop()
			varName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_ISSET_STATIC_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			exists, err := compiler.EvalIssetStaticProp(ctx, classV, varName)
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(exists).ZVal())

		case OpEmptyStaticProp:
			classV := f.pop()
			varName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_EMPTY_STATIC_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			isEmpty, err := compiler.EvalEmptyStaticProp(ctx, classV, varName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(phpv.ZBool(isEmpty).ZVal())

		// --- `$obj->prop++` / `++$obj->prop` / `--` variants ----------
		case OpIncDecObjProp:
			receiver := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_INC_DEC_OBJ_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			inc := ins.B()&1 != 0
			post := ins.B()&2 != 0
			keep := ins.C()&1 != 0
			// PHP 8: ++/-- on a property of a non-object throws
			// "Attempt to increment/decrement property" — distinct
			// from the "Attempt to assign property" thrown by plain
			// property write. Mirrors the r.incDecCtx branch in
			// runObjectVar.Run (compile-object.go:1316).
			if receiver == nil || receiver.Value() == nil {
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to increment/decrement property \"%s\" on null", name))
			}
			if _, isObj := receiver.Value().(phpv.ZObjectAccess); !isObj {
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to increment/decrement property \"%s\" on %s", name, compiler.PhpValueTypeName(receiver)))
			}
			cur, err := objectGet(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			// DoInc mutates in-place — work on a fresh copy.
			cur = cur.Dup()
			var pre *phpv.ZVal
			if post {
				pre = cur.Dup()
			}
			if err := compiler.DoInc(ctx, cur, inc); err != nil {
				return nil, false, err
			}
			if err := objectSet(ctx, receiver, name, cur); err != nil {
				return nil, false, err
			}
			if keep {
				if post {
					f.push(pre)
				} else {
					f.push(cur)
				}
			}

		// --- `Foo::$bar++` / `++Foo::$bar` / `--` variants ------------
		case OpIncDecStaticProp:
			classV := f.pop()
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_INC_DEC_STATIC_PROP name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			inc := ins.B()&1 != 0
			post := ins.B()&2 != 0
			keep := ins.C()&1 != 0
			cur, err := compiler.EvalClassStaticVarRead(ctx, classV, name, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			var pre *phpv.ZVal
			if post {
				pre = cur.Dup()
			}
			if err := compiler.DoInc(ctx, cur, inc); err != nil {
				return nil, false, err
			}
			if err := compiler.AssignClassStaticProp(ctx, classV, name, cur); err != nil {
				return nil, false, err
			}
			if keep {
				if post {
					f.push(pre)
				} else {
					f.push(cur)
				}
			}

		// --- `Foo::$bar OP= rhs` static-prop compound assign --------
		case OpStaticPropCompoundAssign:
			rhs := f.pop()
			classV := f.pop()
			varName, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_STATIC_PROP_COMPOUND_ASSIGN name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			op := tokenizer.ItemType(ins.B())
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: OP_STATIC_PROP_COMPOUND_ASSIGN unknown op %d", ins.B())
			}
			cur, err := compiler.EvalClassStaticVarRead(ctx, classV, varName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			if err := compiler.AssignClassStaticProp(ctx, classV, varName, res); err != nil {
				return nil, false, err
			}
			if ins.C()&1 != 0 {
				f.push(res)
			}

		// --- typed-return non-strict coercion -----------------------
		case OpCoerceReturn:
			ret := f.pop()
			node := f.fn.SubASTs[ins.A()]
			rt := compiler.ReturnTypeOfNode(node)
			ret = compiler.CoerceReturnValue(ctx, ret, rt)
			f.push(ret)

		// --- `new class { … }(args)` anonymous class instantiation --
		case OpNewAnonClass:
			node := f.fn.SubASTs[ins.A()]
			argc := int(ins.B())
			args := make([]*phpv.ZVal, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = f.pop()
			}
			if err := compiler.EnsureAnonClassCompiled(ctx, node); err != nil {
				return nil, false, err
			}
			res, err := compiler.InstantiateAnonClass(ctx, node, args)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `$$name` / `${expr}` variable-variable read ------------
		case OpVarVarRead:
			nameV := f.pop()
			sv, err := nameV.As(ctx, phpv.ZtString)
			if err != nil {
				return nil, false, err
			}
			name := sv.Value().(phpv.ZString)
			warn := ins.A()&1 != 0
			res, err := compiler.LookupVarVar(ctx, name, warn)
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `$$name = v` / `${$expr} = v` variable-variable write ---
		case OpVarVarSet:
			val := f.pop()
			nameV := f.pop()
			if err := compiler.AssignVariableVariable(ctx, nameV, val, f.fn.LocAt(f.pc-1)); err != nil {
				return nil, false, err
			}
			if ins.A()&1 != 0 {
				f.push(val)
			}

		// --- `Cls::${$x}++` / `++Cls::${$x}` dyn-name static-prop inc/dec ---
		case OpIncDecStaticPropDyn:
			nameV := f.pop()
			classV := f.pop()
			name := phpv.ZString(nameV.AsString(ctx))
			inc := ins.B()&1 != 0
			post := ins.B()&2 != 0
			keep := ins.C()&1 != 0
			cur, err := compiler.EvalClassStaticDynVarRead(ctx, classV, name, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			var pre *phpv.ZVal
			if post {
				pre = cur.Dup()
			}
			if err := compiler.DoInc(ctx, cur, inc); err != nil {
				return nil, false, err
			}
			if err := compiler.AssignClassStaticDynProp(ctx, classV, name, cur); err != nil {
				return nil, false, err
			}
			if keep {
				if post {
					f.push(pre)
				} else {
					f.push(cur)
				}
			}

		// --- `Cls::${$x} OP= rhs` dyn-name static-prop compound assign --
		case OpStaticPropDynCompoundAssign:
			rhs := f.pop()
			nameV := f.pop()
			classV := f.pop()
			varName := phpv.ZString(nameV.AsString(ctx))
			op := tokenizer.ItemType(ins.B())
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: OP_STATIC_PROP_DYN_COMPOUND_ASSIGN unknown op %d", ins.B())
			}
			cur, err := compiler.EvalClassStaticDynVarRead(ctx, classV, varName, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			if err := compiler.AssignClassStaticDynProp(ctx, classV, varName, res); err != nil {
				return nil, false, err
			}
			if ins.C()&1 != 0 {
				f.push(res)
			}

		// --- `$b = &$a` ref-assign to simple-local LHS ----------------
		case OpStoreLocalRef:
			v := f.pop()
			idx := ins.A()
			keep := ins.B()&1 != 0
			if v == nil {
				v = phpv.ZNULL.ZVal()
			}
			if v.IsRef() {
				v.RefInner()
			}
			// Write the ref ZVal directly — no Dup, no Nude unwrap.
			// This is the path that storeLocal explicitly avoids
			// (frame.go:160-166 dereferences ref arrays via Nude+Dup).
			f.locals[idx] = v
			if !f.fn.SlotOnly {
				if err := ctx.OffsetSet(ctx, f.fn.Locals[idx], v); err != nil {
					return nil, false, err
				}
			}
			if keep {
				f.push(v)
			}

		// --- `$obj->$x OP= rhs` / `$obj->{$x} OP= rhs` dyn-name prop ---
		case OpObjectDynCompoundAssign:
			rhs := f.pop()
			nameV := f.pop()
			receiver := f.pop()
			name := phpv.ZString(nameV.AsString(ctx))
			op := tokenizer.ItemType(ins.B())
			fn := compoundOp(op)
			if fn == nil {
				return nil, false, fmt.Errorf("vm: OP_OBJECT_DYN_COMPOUND_ASSIGN unknown op %d", ins.B())
			}
			cur, err := objectGet(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, false, err
			}
			if err := objectSet(ctx, receiver, name, res); err != nil {
				return nil, false, err
			}
			if ins.C()&1 != 0 {
				f.push(res)
			}

		// --- `$obj->$x++` / `++$obj->{$x}` dyn-name prop inc/dec -----
		case OpIncDecObjDynProp:
			nameV := f.pop()
			receiver := f.pop()
			name := phpv.ZString(nameV.AsString(ctx))
			inc := ins.B()&1 != 0
			post := ins.B()&2 != 0
			keep := ins.C()&1 != 0
			// PHP 8: ++/-- on a property of a non-object throws
			// "Attempt to increment/decrement property" — mirrors the
			// static-name OP_INC_DEC_OBJ_PROP receiver check above.
			if receiver == nil || receiver.Value() == nil {
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to increment/decrement property \"%s\" on null", name))
			}
			if _, isObj := receiver.Value().(phpv.ZObjectAccess); !isObj {
				return nil, false, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Attempt to increment/decrement property \"%s\" on %s", name, compiler.PhpValueTypeName(receiver)))
			}
			cur, err := objectGet(ctx, receiver, name)
			if err != nil {
				return nil, false, err
			}
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			cur = cur.Dup()
			var pre *phpv.ZVal
			if post {
				pre = cur.Dup()
			}
			if err := compiler.DoInc(ctx, cur, inc); err != nil {
				return nil, false, err
			}
			if err := objectSet(ctx, receiver, name, cur); err != nil {
				return nil, false, err
			}
			if keep {
				if post {
					f.push(pre)
				} else {
					f.push(cur)
				}
			}

		// --- runConstant — user / namespaced / built-in constant ----
		case OpLoadConstantByName:
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, false, fmt.Errorf("vm: OP_LOAD_CONSTANT_BY_NAME name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			noFallback := ins.B()&1 != 0
			res, err := compiler.LookupConstant(ctx, string(name), noFallback, f.fn.LocAt(f.pc-1))
			if err != nil {
				return nil, false, err
			}
			f.push(res)

		// --- `unset($localVar)` ------------------------------------
		case OpUnsetLocal:
			idx := ins.A()
			v := f.locals[idx]
			f.locals[idx] = nil
			if v != nil {
				if err := compiler.CallDestructorIfNeeded(ctx, v); err != nil {
					return nil, false, err
				}
			}
			if !f.fn.SlotOnly {
				if err := ctx.OffsetUnset(ctx, f.fn.Locals[idx]); err != nil {
					return nil, false, err
				}
			}

		// --- array spread entry: `[…, ...$expr, …]` -----------------
		case OpArraySpreadAppend:
			v := f.pop()
			arr := f.peek().AsArray(ctx)
			if err := compiler.SpreadIntoArray(ctx, arr, v); err != nil {
				return nil, false, err
			}

		// --- top-level `const NAME = expr;` definition --------------
		case OpDefineConst:
			value := f.pop()
			node := f.fn.SubASTs[ins.A()]
			name, _, attrs, loc := compiler.TopLevelConstParts(node)
			if err := compiler.DefineTopLevelConst(ctx, name, value, attrs, loc); err != nil {
				return nil, false, err
			}

		default:
			return nil, false, fmt.Errorf("%w: %d at pc=%d", ErrUnknownOp, ins.Op(), f.pc-1)
		}
	}
}

// binop pops b then a, computes fn(ctx, a, b), pushes result.
func (f *Frame) binop(ctx phpv.Context, fn func(phpv.Context, *phpv.ZVal, *phpv.ZVal) (*phpv.ZVal, error)) error {
	b := f.pop()
	a := f.pop()
	res, err := fn(ctx, a, b)
	if err != nil {
		return err
	}
	f.push(res)
	return nil
}

// unop pops, computes fn(ctx, top), pushes result.
func (f *Frame) unop(ctx phpv.Context, fn func(phpv.Context, *phpv.ZVal) (*phpv.ZVal, error)) error {
	a := f.pop()
	res, err := fn(ctx, a)
	if err != nil {
		return err
	}
	f.push(res)
	return nil
}

// intIntFast returns (ai, bi, true) when both stack top operands are
// ZtInt; the dispatch loop can then compute the result inline. Returns
// (_, _, false) otherwise — the caller falls through to the generic
// binop path.
func (f *Frame) intIntFast() (ai, bi phpv.ZInt, ok bool) {
	b := f.stack[f.sp-1]
	a := f.stack[f.sp-2]
	if a.GetType() != phpv.ZtInt || b.GetType() != phpv.ZtInt {
		return 0, 0, false
	}
	return a.Value().(phpv.ZInt), b.Value().(phpv.ZInt), true
}

// pushBoolReplace2 replaces the top two stack slots with a single bool
// — the common shape of an int-int compare fast path.
func (f *Frame) pushBoolReplace2(v bool) {
	if v {
		f.stack[f.sp-2] = phpv.ZTrue.ZVal()
	} else {
		f.stack[f.sp-2] = phpv.ZFalse.ZVal()
	}
	f.sp--
}

// incDecLocal applies ++/-- to the local at index, optionally pushing
// the pre-mutation value (post=true) or the post-mutation value
// (post=false). Mirrors the AST's runOperator behaviour for ++/-- on a
// simple variable target. Reads/writes go through the slot cache.
func (f *Frame) incDecLocal(ctx phpv.Context, idx uint16, inc bool, post bool) error {
	v := f.locals[idx]
	if v == nil {
		// Undefined-variable warning matches the AST runVariable.Run
		// behaviour for an unwritten ++ / -- target. ZNULL.ZVal() →
		// DoInc → ZInt(1) / ZInt(-1) per PHP semantics.
		name := f.fn.Locals[idx]
		if name != "this" {
			if err := ctx.Warn("Undefined variable $%s", string(name), logopt.NoFuncName(true)); err != nil {
				return err
			}
		}
		v = phpv.ZNULL.ZVal()
	}
	// DoInc mutates v in-place; cached singletons must not be mutated,
	// so dup first. (After this dup, the slot points to the fresh
	// non-cached ZVal once we storeLocal below.)
	if v.IsCached() {
		v = phpv.NewZVal(v.Value())
	}
	var pre *phpv.ZVal
	if post {
		pre = v.Dup()
	}
	if err := compiler.DoInc(ctx, v, inc); err != nil {
		return err
	}
	if err := f.storeLocal(ctx, idx, v); err != nil {
		return err
	}
	if post {
		f.push(pre)
	} else {
		f.push(v)
	}
	return nil
}

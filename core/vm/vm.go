package vm

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phperr"
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
	f := &Frame{
		fn:     fn,
		stack:  make([]Slot, fn.MaxStack),
		locals: make([]*phpv.ZVal, len(fn.Locals)),
	}
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

// exec is the dispatch loop. Returns whatever value OpRet popped, or
// nil if execution falls off the end without a return (top-level
// scripts).
func (f *Frame) exec(ctx phpv.Context) (res *phpv.ZVal, err error) {
	defer func() {
		// Wrap non-PhpThrow errors with the active loc, mirroring AST
		// behaviour (r.l.Error(ctx, err)). PhpThrow propagates as-is so
		// try/catch sees the original exception object.
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
		// Find the active loc for the offending PC. f.pc has already
		// been incremented past the failing instruction, so look up at
		// pc-1 when possible.
		pc := f.pc
		if pc > 0 {
			pc--
		}
		if loc := f.fn.LocAt(pc); loc != nil {
			err = loc.Error(ctx, err)
		}
	}()

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
				v = phpv.ZNULL.ZVal()
			}
			f.push(v)

		case OpLoadLocalOrWarn:
			v := f.locals[ins.A()]
			if v == nil {
				name := f.fn.Locals[ins.A()]
				if err := ctx.Warn("Undefined variable $%s", string(name), logopt.NoFuncName(true)); err != nil {
					return nil, err
				}
				v = phpv.ZNULL.ZVal()
			}
			f.push(v)

		case OpStoreLocal:
			v := f.pop()
			if err := f.storeLocal(ctx, ins.A(), v); err != nil {
				return nil, err
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
				return nil, err
			}

		case OpDup:
			f.push(f.peek())

		case OpPop:
			f.pop()

		// --- arithmetic / bitwise -----------------------------------
		case OpAdd:
			if err := f.binop(ctx, opAdd); err != nil {
				return nil, err
			}
		case OpSub:
			if err := f.binop(ctx, opSub); err != nil {
				return nil, err
			}
		case OpMul:
			if err := f.binop(ctx, opMul); err != nil {
				return nil, err
			}
		case OpDiv:
			if err := f.binop(ctx, opDiv); err != nil {
				return nil, err
			}
		case OpMod:
			if err := f.binop(ctx, opMod); err != nil {
				return nil, err
			}
		case OpPow:
			if err := f.binop(ctx, opPow); err != nil {
				return nil, err
			}
		case OpBitAnd:
			if err := f.binop(ctx, opBitAnd); err != nil {
				return nil, err
			}
		case OpBitOr:
			if err := f.binop(ctx, opBitOr); err != nil {
				return nil, err
			}
		case OpBitXor:
			if err := f.binop(ctx, opBitXor); err != nil {
				return nil, err
			}
		case OpShl:
			if err := f.binop(ctx, opShl); err != nil {
				return nil, err
			}
		case OpShr:
			if err := f.binop(ctx, opShr); err != nil {
				return nil, err
			}
		case OpConcat:
			if err := f.binop(ctx, opConcat); err != nil {
				return nil, err
			}

		case OpNeg:
			if err := f.unop(ctx, opNeg); err != nil {
				return nil, err
			}
		case OpBitNot:
			if err := f.unop(ctx, opBitNot); err != nil {
				return nil, err
			}
		case OpNot:
			if err := f.unop(ctx, opNot); err != nil {
				return nil, err
			}

		// --- compare -------------------------------------------------
		case OpCmpEq:
			if err := f.binop(ctx, opCmpEq); err != nil {
				return nil, err
			}
		case OpCmpNe:
			if err := f.binop(ctx, opCmpNe); err != nil {
				return nil, err
			}
		case OpCmpId:
			if err := f.binop(ctx, opCmpId); err != nil {
				return nil, err
			}
		case OpCmpNid:
			if err := f.binop(ctx, opCmpNid); err != nil {
				return nil, err
			}
		case OpCmpLt:
			if err := f.binop(ctx, opCmpLt); err != nil {
				return nil, err
			}
		case OpCmpLe:
			if err := f.binop(ctx, opCmpLe); err != nil {
				return nil, err
			}
		case OpCmpGt:
			if err := f.binop(ctx, opCmpGt); err != nil {
				return nil, err
			}
		case OpCmpGe:
			if err := f.binop(ctx, opCmpGe); err != nil {
				return nil, err
			}
		case OpCmpSpaceship:
			if err := f.binop(ctx, opCmpSpaceship); err != nil {
				return nil, err
			}

		case OpBool:
			v := f.peek()
			f.replaceTop(phpv.ZBool(v.AsBool(ctx)).ZVal())

		// --- inc/dec on locals --------------------------------------
		case OpIncLocal:
			if err := f.incDecLocal(ctx, ins.A(), true, false); err != nil {
				return nil, err
			}
		case OpDecLocal:
			if err := f.incDecLocal(ctx, ins.A(), false, false); err != nil {
				return nil, err
			}
		case OpPostIncLocal:
			if err := f.incDecLocal(ctx, ins.A(), true, true); err != nil {
				return nil, err
			}
		case OpPostDecLocal:
			if err := f.incDecLocal(ctx, ins.A(), false, true); err != nil {
				return nil, err
			}

		// --- compound assign on locals ------------------------------
		case OpOpAssignLocal:
			fn := compoundOp(tokenizer.ItemType(ins.B()))
			if fn == nil {
				return nil, fmt.Errorf("vm: unknown compound op %d", ins.B())
			}
			rhs := f.pop()
			cur := f.locals[ins.A()]
			if cur == nil {
				cur = phpv.ZNULL.ZVal()
			}
			res, err := fn(ctx, cur, rhs)
			if err != nil {
				return nil, err
			}
			if err := f.storeLocal(ctx, ins.A(), res); err != nil {
				return nil, err
			}

		// --- control flow -------------------------------------------
		case OpJmp:
			f.pc = uint32(int32(f.pc) + ins.C())

		case OpJmpIfFalse:
			v := f.pop()
			if !bool(v.AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfTrue:
			v := f.pop()
			if bool(v.AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfFalsePeek:
			if !bool(f.peek().AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		case OpJmpIfTruePeek:
			if bool(f.peek().AsBool(ctx)) {
				f.pc = uint32(int32(f.pc) + ins.C())
			}

		// --- call ----------------------------------------------------
		case OpCallUser:
			name, ok := f.fn.Consts[ins.A()].(phpv.ZString)
			if !ok {
				return nil, fmt.Errorf("vm: OP_CALL_USER name const is %T not ZString", f.fn.Consts[ins.A()])
			}
			argc := int(ins.B())
			if err := f.callUser(ctx, name, argc); err != nil {
				return nil, err
			}
			// callUser pops argc and pushes 1 result.

		// --- return --------------------------------------------------
		case OpRet:
			return f.pop(), nil

		case OpRetNull:
			return phpv.ZNULL.ZVal(), nil

		// --- arrays --------------------------------------------------
		case OpNewArray:
			arr := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
			f.push(arr.ZVal())

		case OpArrayInitAppend:
			val := f.pop()
			arr := f.peek().AsArray(ctx)
			if err := arr.OffsetSet(ctx, nil, val); err != nil {
				return nil, err
			}

		case OpArrayInitKeyed:
			val := f.pop()
			key := f.pop()
			arr := f.peek().AsArray(ctx)
			if err := arr.OffsetSet(ctx, key.Value(), val); err != nil {
				return nil, err
			}

		case OpArrayGet:
			offset := f.pop()
			container := f.pop()
			res, err := arrayGet(ctx, container, offset)
			if err != nil {
				return nil, err
			}
			if res == nil {
				res = phpv.ZNULL.ZVal()
			}
			f.push(res)

		case OpArraySetLocal:
			val := f.pop()
			offset := f.pop()
			if err := arraySetLocal(ctx, f, ins.A(), offset, val); err != nil {
				return nil, err
			}

		case OpArrayAppendLocal:
			val := f.pop()
			if err := arraySetLocal(ctx, f, ins.A(), nil, val); err != nil {
				return nil, err
			}

		// --- foreach -------------------------------------------------
		case OpForeachInit:
			src := f.pop()
			it, err := foreachInit(ctx, src)
			if err != nil {
				return nil, err
			}
			if it == nil {
				// Not iterable: emit warning and jump past the loop.
				if err := foreachWarnInvalid(ctx, src, f.fn.LocAt(f.pc-1)); err != nil {
					return nil, err
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
				return nil, err
			}
			f.locals[ins.A()] = cur
			valLocal := f.fn.Locals[ins.A()]
			if err := ctx.OffsetSet(ctx, valLocal, cur); err != nil {
				return nil, err
			}
			if ins.B() != 0xFFFF {
				key, err := it.Key(ctx)
				if err != nil {
					return nil, err
				}
				f.locals[ins.B()] = key
				keyLocal := f.fn.Locals[ins.B()]
				if err := ctx.OffsetSet(ctx, keyLocal, key); err != nil {
					return nil, err
				}
			}

		case OpForeachAdvance:
			it := f.iters[len(f.iters)-1]
			if _, err := it.Next(ctx); err != nil {
				return nil, err
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
				return nil, err
			}
			ctx.Global().DrainTempObjects()

		default:
			return nil, fmt.Errorf("%w: %d at pc=%d", ErrUnknownOp, ins.Op(), f.pc-1)
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

// incDecLocal applies ++/-- to the local at index, optionally pushing
// the pre-mutation value (post=true) or the post-mutation value
// (post=false). Mirrors the AST's runOperator behaviour for ++/-- on a
// simple variable target. Reads/writes go through the slot cache.
func (f *Frame) incDecLocal(ctx phpv.Context, idx uint16, inc bool, post bool) error {
	v := f.locals[idx]
	if v == nil {
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

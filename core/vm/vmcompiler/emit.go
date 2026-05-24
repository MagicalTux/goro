package vmcompiler

import (
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// emitter accumulates state while walking the AST. It is created per
// Compile invocation and discarded after. Not safe for concurrent use.
type emitter struct {
	ctx      phpv.Context // optional — used for callable-introspection lookups
	code     []vm.Instruction
	consts   []phpv.Val
	constMap map[phpv.Val]uint16

	locals   []phpv.ZString
	localMap map[phpv.ZString]uint16

	subFns      []*vm.Function
	subClosures []phpv.Runnable
	subASTs     []phpv.Runnable
	subArgs     [][]phpv.Runnable
	tryHandlers []vm.TryHandler

	locs []vm.LocEntry

	// Loop context for break/continue patches.
	loops []*loopCtx

	// finallyLoopDepths records len(e.loops) at the moment each
	// active try-with-finally body started emitting. emitBreak /
	// emitContinue check the innermost entry: if the loop being
	// targeted sits below that depth, the jump would skip a finally,
	// which the native lowering doesn't support yet — the emitter
	// falls back via unsupportedf so the whole function routes
	// through the AST.
	finallyLoopDepths []int

	// Source location of the unit currently being compiled.
	source *phpv.Loc

	// Statement-context flag — true when emitting a statement whose
	// result is discarded. Currently informational; future passes can
	// use it to elide DUPs and other intermediates.
	stmtCtx bool

	// curStack tracks the simulated stack depth so we can compute
	// MaxStack without a separate analysis pass. push/pop helpers
	// update it.
	curStack int
	maxStack int

	// synthSeq mints unique sequence numbers for synthetic local
	// names (e.g. __switch_cond_N) so nested switches don't share
	// a slot.
	synthSeq int
}

// nextSynthID returns a fresh integer for synthesising unique local
// names in nested constructs.
func (e *emitter) nextSynthID() int {
	id := e.synthSeq
	e.synthSeq++
	return id
}

type loopCtx struct {
	// pcs of OP_JMP placeholders that should jump to the loop's exit.
	breakPCs []uint32
	// pcs of OP_JMP placeholders that should jump to the loop's
	// step/condition (i.e. the natural continue target).
	continuePCs []uint32
	// isForeach marks foreach loops so multi-level break/continue
	// can emit OP_FOREACH_UNWIND for each foreach being skipped over.
	isForeach bool
}

func newEmitter(source *phpv.Loc) *emitter {
	return &emitter{
		constMap: map[phpv.Val]uint16{},
		localMap: map[phpv.ZString]uint16{},
		source:   source,
	}
}

// emit appends an instruction and returns its PC.
func (e *emitter) emit(op vm.Op, a, b uint16, c int32) uint32 {
	pc := uint32(len(e.code))
	e.code = append(e.code, vm.Encode(op, a, b, c))
	return pc
}

// patchJump rewrites the C field of an emitted JMP-style instruction
// at jumpPC so it lands at target. Uses VM jump semantics: after fetch,
// f.pc points one past the jump; pc += C lands at target.
func (e *emitter) patchJump(jumpPC, target uint32) {
	c := int32(target) - int32(jumpPC+1)
	op := e.code[jumpPC].Op()
	a := e.code[jumpPC].A()
	b := e.code[jumpPC].B()
	e.code[jumpPC] = vm.Encode(op, a, b, c)
}

// constIndex returns the const-pool index for v, deduplicating.
// For values that aren't comparable (e.g. ZArray pointers we don't
// embed today), the caller should fall back to ErrUnsupported earlier.
func (e *emitter) constIndex(v phpv.Val) uint16 {
	if idx, ok := e.constMap[v]; ok {
		return idx
	}
	idx := uint16(len(e.consts))
	e.consts = append(e.consts, v)
	e.constMap[v] = idx
	return idx
}

// closureIndex registers a *ZClosure-shaped Runnable in the SubClosures
// table and returns its index. No deduplication — each call site gets
// its own slot so we don't mix per-invocation captures.
func (e *emitter) closureIndex(r phpv.Runnable) uint16 {
	idx := uint16(len(e.subClosures))
	e.subClosures = append(e.subClosures, r)
	return idx
}

// astIndex registers an arbitrary AST Runnable in the SubASTs table.
// Used by opcodes that delegate evaluation to the AST runner for
// nodes too intricate to lower piecewise (class const, …).
func (e *emitter) astIndex(r phpv.Runnable) uint16 {
	idx := uint16(len(e.subASTs))
	e.subASTs = append(e.subASTs, r)
	return idx
}

// subArgsIndex registers a call-site argument expression list in the
// SubArgs table. Used by the "ByExprs" call opcodes which hand the raw
// AST argument expressions to ctx.Call so the binding layer can apply
// by-ref / named / spread semantics. No dedup — each call site gets
// its own slot.
func (e *emitter) subArgsIndex(args []phpv.Runnable) uint16 {
	idx := uint16(len(e.subArgs))
	e.subArgs = append(e.subArgs, args)
	return idx
}

// localIndex returns the local-table index for the named variable,
// adding it on first use.
func (e *emitter) localIndex(name phpv.ZString) uint16 {
	if idx, ok := e.localMap[name]; ok {
		return idx
	}
	idx := uint16(len(e.locals))
	e.locals = append(e.locals, name)
	e.localMap[name] = idx
	return idx
}

// recordLoc adds a location entry for the next emitted instruction.
// Idempotent: a duplicate consecutive entry is suppressed.
func (e *emitter) recordLoc(loc *phpv.Loc) {
	if loc == nil {
		return
	}
	pc := uint32(len(e.code))
	if n := len(e.locs); n > 0 && e.locs[n-1].Loc == loc {
		return
	}
	e.locs = append(e.locs, vm.LocEntry{PC: pc, Loc: loc})
}

// pushStack/popStack track simulated stack depth.
func (e *emitter) pushStack(n int) {
	e.curStack += n
	if e.curStack > e.maxStack {
		e.maxStack = e.curStack
	}
}

func (e *emitter) popStack(n int) {
	e.curStack -= n
	if e.curStack < 0 {
		// emitter bug: stack underflow during compilation.
		panic("vmcompiler: stack underflow during emit")
	}
}

// finish builds the final *vm.Function. Always call this last.
func (e *emitter) finish(name phpv.ZString, numParams int) *vm.Function {
	cached := make([]*phpv.ZVal, len(e.consts))
	for i, c := range e.consts {
		cached[i] = phpv.MakeCachedZVal(c)
	}
	maxStack := e.maxStack
	if maxStack < 4 {
		maxStack = 4
	}
	src := e.source
	if src == nil {
		src = &phpv.Loc{Filename: "<vm>"}
	}
	return &vm.Function{
		Code:        e.code,
		Consts:      e.consts,
		CachedZ:     cached,
		Locals:      e.locals,
		SubFns:      e.subFns,
		SubClosures: e.subClosures,
		SubASTs:     e.subASTs,
		SubArgs:     e.subArgs,
		LocsSparse:  e.locs,
		TryHandlers: e.tryHandlers,
		NumParams:   numParams,
		MaxStack:    maxStack,
		Source:      src,
		Name:        name,
	}
}

// pushLoop opens a new loop context (for break/continue patches).
func (e *emitter) pushLoop() *loopCtx {
	c := &loopCtx{}
	e.loops = append(e.loops, c)
	return c
}

// popLoop closes the most recent loop context.
func (e *emitter) popLoop() *loopCtx {
	n := len(e.loops)
	c := e.loops[n-1]
	e.loops = e.loops[:n-1]
	return c
}

// curLoop returns the most recent loop context, or nil if not in one.
func (e *emitter) curLoop() *loopCtx {
	if len(e.loops) == 0 {
		return nil
	}
	return e.loops[len(e.loops)-1]
}

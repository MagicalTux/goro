package vm

import (
	"sort"

	"github.com/KarpelesLab/goro/core/phpv"
)

// LocEntry maps a starting program counter to a source location.
// The Function carries a sparse, sorted list of these — at error or
// tick time the VM looks up the active loc with LocAt(pc).
type LocEntry struct {
	PC  uint32
	Loc *phpv.Loc
}

// Function is a single VM-compiled unit (top-level script body or a
// user-defined function). It is immutable after construction.
type Function struct {
	Code        []Instruction
	Consts      []phpv.Val   // const-pool — literals + interned ZString names
	CachedZ     []*phpv.ZVal // pre-built MakeCachedZVal(Consts[i]) for OP_LOAD_CONST
	Locals      []phpv.ZString
	SubFns      []*Function     // direct-call targets (resolved at emit time)
	SubClosures []phpv.Runnable // *ZClosure templates referenced by OP_MAKE_CLOSURE
	SubASTs     []phpv.Runnable // delegated AST nodes (class const, static, …)
	// SubArgs holds per-call-site argument expression lists for the
	// "ByExprs" call opcodes (OP_CALL_USER_BY_EXPRS, OP_CALL_INDIRECT_BY_EXPRS,
	// OP_OBJECT_CALL_BY_EXPRS). The runtime passes args[idx] to ctx.Call,
	// which evaluates each expression in caller scope with full by-ref /
	// named / spread support — the path the AST runner's
	// runnableFunctionCall.Run takes. Indexed by the opcode's B field.
	SubArgs [][]phpv.Runnable
	LocsSparse  []LocEntry      // sorted by PC
	TryHandlers []TryHandler
	NumParams   int
	MaxStack    int
	Source      *phpv.Loc
	Name        phpv.ZString
	// SlotOnly enables the slot-only-write optimization: local writes
	// skip the FuncContext hashtable mirror. Set by the emitter when
	// the function body has no extract/compact/get_defined_vars/$$x.
	// External callers (call_user_func_array, debug_backtrace arg
	// lists, etc.) may still see stale parameter values when this is
	// on, so the emitter is conservative.
	SlotOnly bool

	// CallableCache is a parallel slice to Consts: when index i holds
	// a name used by OP_CALL_USER, the resolved callable is cached at
	// CallableCache[i] after the first call. Cleared back to nil if a
	// later call sees a different callable (function redeclared).
	// Lazy-allocated on first cache write.
	CallableCache []phpv.Callable
}

// TryHandler describes a `try { … } catch …` region. The dispatcher
// scans handlers on PhpThrow: the innermost (last-registered) handler
// whose [Start, End) range contains the failing PC wins. There is no
// per-instruction encoding — handlers are static metadata.
type TryHandler struct {
	Start, End uint32 // PC range of the try body (exclusive End)
	// AfterCatch is the PC immediately after the last catch body —
	// i.e. where execution resumes when a catch body either runs to
	// completion or a destructor that fired while binding the catch
	// variable throws and overrides the in-flight exception (bug53511).
	// Setting f.pc to AfterCatch before propagating the destructor's
	// throw stops dispatchTryHandler from re-matching the same try.
	AfterCatch uint32
	// StackBase is the simulated stack depth at try entry. The
	// dispatcher resets sp to this value when unwinding to a catch
	// body so partially-pushed arguments from the failing expression
	// are dropped.
	StackBase int
	Catches   []CatchClause
	// HasFinally is true when this try clause has a finally body.
	// When set, FinallyPC is the PC of the finally body's first
	// instruction and FinallyEnd is the PC of its OP_FINALLY_END
	// marker. The finally-protected region for the purposes of
	// return / break / throw routing is [Start, FinallyPC).
	HasFinally bool
	FinallyPC  uint32
	FinallyEnd uint32
}

// CatchClause is a single `catch (TypeA | TypeB $e)` block.
type CatchClause struct {
	// Types are the type names declared in the catch. Empty types means
	// `catch (Throwable)` — i.e. matches every exception.
	Types []phpv.ZString
	// VarIdx is the local index for the bound exception. 0xFFFF means
	// the catch has no variable: `catch (Exception)`.
	VarIdx uint16
	// PC is the start of the catch body.
	PC uint32
	// Loc is the source location of the `catch` keyword. PHP attributes
	// destructor stack frames to this line when a destructor fires
	// while binding the catch variable (bug53511).
	Loc *phpv.Loc
}


// LocAt returns the active source location for the instruction at pc.
// If no entry covers pc it returns Source (the function's declaration
// location), which is always non-nil for emitted Functions.
func (f *Function) LocAt(pc uint32) *phpv.Loc {
	if len(f.LocsSparse) == 0 {
		return f.Source
	}
	// Binary search for the largest PC <= pc.
	idx := sort.Search(len(f.LocsSparse), func(i int) bool {
		return f.LocsSparse[i].PC > pc
	})
	if idx == 0 {
		return f.Source
	}
	return f.LocsSparse[idx-1].Loc
}

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
	SubFns      []*Function // direct-call targets (resolved at emit time)
	LocsSparse  []LocEntry  // sorted by PC
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
}

// TryHandler describes a `try { … } catch …` region. The dispatcher
// scans handlers on PhpThrow: the innermost (last-registered) handler
// whose [Start, End) range contains the failing PC wins. There is no
// per-instruction encoding — handlers are static metadata.
type TryHandler struct {
	Start, End uint32 // PC range of the try body (exclusive End)
	Catches    []CatchClause
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

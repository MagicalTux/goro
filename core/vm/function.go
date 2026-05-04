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
	Code       []Instruction
	Consts     []phpv.Val   // const-pool — literals + interned ZString names
	CachedZ    []*phpv.ZVal // pre-built MakeCachedZVal(Consts[i]) for OP_LOAD_CONST
	Locals     []phpv.ZString
	SubFns     []*Function   // direct-call targets (resolved at emit time)
	LocsSparse []LocEntry    // sorted by PC
	NumParams  int
	MaxStack   int
	Source     *phpv.Loc
	Name       phpv.ZString
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

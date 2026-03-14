package logopt

type ErrType int

type NoFuncName bool

type NoLoc bool

type IsInternal bool

type Data struct {
	ErrType    int
	NoFuncName bool
	NoLoc      bool
	IsInternal bool // marks error as originating from internal (non-userspace) code
	Loc        any  // optional *phpv.Loc override for error location
}

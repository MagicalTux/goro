package logopt

type ErrType int

type NoFuncName bool

type NoLoc bool

type Data struct {
	ErrType    int
	NoFuncName bool
	NoLoc      bool
	Loc        any // optional *phpv.Loc override for error location
}

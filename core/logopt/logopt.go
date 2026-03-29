package logopt

type ErrType int

type NoFuncName bool

type NoLoc bool

type IsInternal bool

// LocNewLine causes the " in FILE on line N" location suffix to be output
// on a new line with a leading space, instead of appended to the same line.
// Used for messages that already contain their own "in X on line N" info
// (e.g. INI parser errors).
type LocNewLine bool

type Data struct {
	ErrType    int
	NoFuncName bool
	NoLoc      bool
	IsInternal bool // marks error as originating from internal (non-userspace) code
	LocNewLine bool // put PHP location on a new line (for messages with embedded location)
	Loc        any  // optional *phpv.Loc override for error location
}

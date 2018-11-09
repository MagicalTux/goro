package core

type Loc struct {
	Filename   string
	Line, Char int
}

func MakeLoc(Filename string, Line, Char int) *Loc {
	return &Loc{Filename, Line, Char}
}

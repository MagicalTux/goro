package core

import "fmt"

type Loc struct {
	Filename   string
	Line, Char int
}

func (l *Loc) Loc() *Loc {
	return l
}

func (l *Loc) Run(ctx Context) (*ZVal, error) {
	// just a checkpoint, do nothing
	return nil, nil
}

func MakeLoc(Filename string, Line, Char int) *Loc {
	return &Loc{Filename, Line, Char}
}

func (l *Loc) Error(e error) *PhpError {
	switch err := e.(type) {
	case *PhpError:
		if err.l == nil {
			err.l = l
		}
		return err
	default:
		return &PhpError{e, l, PhpErrorFatal}
	}
}

func (l *Loc) Errorf(f string, arg ...interface{}) *PhpError {
	return &PhpError{fmt.Errorf(f, arg...), l, PhpErrorFatal}
}

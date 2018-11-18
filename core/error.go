package core

import (
	"fmt"
	"io"
)

type PhpErrorType int

const (
	PhpErrorFatal = iota
	PhpThrow
	PhpBreak
	PhpContinue
	PhpExit
)

type PhpError struct {
	e error
	l *Loc
	t PhpErrorType

	intv ZInt
	obj  *ZObject // if PhpThrow
}

func (e *PhpError) Run(ctx Context) (*ZVal, error) {
	return nil, e
}

func (e *PhpError) Loc() *Loc {
	return e.l
}

func (e *PhpError) Dump(w io.Writer) error {
	switch e.t {
	case PhpBreak:
		_, err := w.Write([]byte("break"))
		return err
	case PhpContinue:
		_, err := w.Write([]byte("continue"))
		return err
	case PhpExit:
		_, err := fmt.Fprintf(w, "exit(%d)", e.intv)
		return err
	default:
		_, err := fmt.Fprintf(w, "TODO") // TODO
		return err
	}
}

func (e *PhpError) Error() string {
	if e.l == nil {
		if e.e == nil {
			return "Unknown error " + debugDump(e)
		}
		return e.e.Error()
	}
	return fmt.Sprintf("%s in %s on line %d", e.e, e.l.Filename, e.l.Line)
}

func ExitError(retcode ZInt) *PhpError {
	return &PhpError{t: PhpExit, intv: retcode}
}

func (e *PhpError) IsExit() bool {
	return e.t == PhpExit
}

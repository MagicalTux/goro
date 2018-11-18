package core

import (
	"fmt"
	"io"
)

type PhpErrorType int

const (
	E_ERROR PhpErrorType = 1 << iota
	E_WARNING
	E_PARSE
	E_NOTICE
	E_CORE_ERROR
	E_CORE_WARNING
	E_COMPILE_ERROR
	E_COMPILE_WARNING
	E_USER_ERROR
	E_USER_WARNING
	E_USER_NOTICE
	E_STRICT
	E_RECOVERABLE_ERROR
	E_DEPRECATED
	E_USER_DEPRECATED
	E_ALL PhpErrorType = (1 << iota) - 1
)

type PhpError struct {
	e error
	l *Loc

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
	_, err := fmt.Fprintf(w, "TODO") // TODO
	return err
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

func (e *PhpError) IsExit() bool {
	_, r := e.e.(*PhpExit)
	return r
}

package phpv

import (
	"fmt"
	"io"
)

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

func (l *Loc) Dump(w io.Writer) error {
	return nil
}

func (l *Loc) Error(e error, codeArg ...PhpErrorType) *PhpError {
	code := E_ERROR
	if len(codeArg) > 0 {
		code = codeArg[0]
	}
	// fill location if missing
	switch err := e.(type) {
	case *PhpError:
		if err.Loc == nil {
			err.Loc = l
		}
		return err
	default:
		return &PhpError{Err: e, Code: code, Loc: l}
	}
}

func (l *Loc) Errorf(code PhpErrorType, f string, arg ...interface{}) *PhpError {
	return &PhpError{Err: fmt.Errorf(f, arg...), Loc: l, Code: code}
}

func (l *Loc) String() string {
	return fmt.Sprintf("at %s on line %d", l.Filename, l.Line)
}

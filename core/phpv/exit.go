package phpv

import (
	"fmt"
	"io"
)

type PhpExit struct {
	l    *Loc
	intv ZInt
}

func (e *PhpExit) Run(ctx Context) (*ZVal, error) {
	return nil, e
}

func (e *PhpExit) Error() string {
	return "Program exitted"
}

func (e *PhpExit) Loc() *Loc {
	return e.l
}

func (e *PhpExit) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "exit(%d)", e.intv)
	return err
}

func ExitError(retcode ZInt) error {
	return &PhpExit{intv: retcode}
}

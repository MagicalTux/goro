package phpv

import (
	"fmt"
	"io"
)

// PhpExit is returned as a Go error when PHP code invokes exit()/die().
// Library users can type-assert returned errors to *PhpExit (or use errors.As)
// to retrieve the exit code and decide how to terminate — goro itself does
// NOT call os.Exit, which is the responsibility of the embedding SAPI.
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

// Code returns the exit code passed to exit()/die(). For exit() called
// without an argument or with a non-int value, this is 0.
func (e *PhpExit) Code() int {
	return int(e.intv)
}

// ExitError builds a PhpExit error with the given return code. This is the
// error that exit()/die() produce; embedders can detect it via errors.As
// or a type assertion on the error returned from RunFile / script execution.
func ExitError(retcode ZInt) error {
	return &PhpExit{intv: retcode}
}

package phpv

import (
	"bytes"
	"fmt"
	"os"
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
	Err      error
	FuncName string
	Code     PhpErrorType
	Loc      *Loc

	PhpStackTrace StackTrace
	GoStackTrace  []byte
}

func (e *PhpError) CanBeUserHandled() bool {
	switch e.Code {
	case E_ERROR, E_PARSE, E_CORE_ERROR,
		E_CORE_WARNING, E_COMPILE_ERROR,
		E_COMPILE_WARNING, E_STRICT:
		return false
	}
	return true
}

func (e *PhpError) IsNonFatal() bool {
	switch e.Code {
	case E_WARNING, E_USER_WARNING, E_USER_NOTICE, E_DEPRECATED, E_USER_DEPRECATED:
		return true
	}
	return false
}

func (e *PhpError) Error() string {
	if e.Loc == nil {
		if e.Err == nil {
			return "Unknown error"
		}
		return e.Err.Error()
	}
	var name string
	if e.FuncName != "" {
		name = e.FuncName + "(): "
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s%s in %s:%d", name, e.Err, e.Loc.Filename, e.Loc.Line))
	buf.WriteByte('\n')
	buf.WriteString("Stack trace:")
	buf.WriteByte('\n')
	buf.WriteString(e.PhpStackTrace.String().String())
	buf.WriteByte('\n')
	buf.WriteString(fmt.Sprintf("  thrown in %s on line %d", e.Loc.Filename, e.Loc.Line))
	if os.Getenv("DEBUG") != "" {
		buf.WriteByte('\n')
		buf.Write(e.GoStackTrace)
	}

	return buf.String()
}

func (e *PhpError) IsExit() bool {
	_, r := e.Err.(*PhpExit)
	return r
}

func FilterError(err error) error {
	// check for stuff like PhpExit and filter out
	switch e := err.(type) {
	case *PhpExit:
		return nil
	case *PhpError:
		switch e.Err.(type) {
		case *PhpExit:
			return nil
		}
	}
	return err
}

package phpv

import "fmt"

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
	return fmt.Sprintf("%s%s in %s on line %d", name, e.Err, e.Loc.Filename, e.Loc.Line)
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

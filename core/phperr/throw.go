package phperr

import (
	"github.com/MagicalTux/goro/core/phpv"
)

type PhpThrow struct {
	Obj phpv.ZObject
	Loc *phpv.Loc
}

// ThrownFile returns the file where the exception was constructed (from the
// exception object's "file" property), falling back to Loc.Filename.
func (e *PhpThrow) ThrownFile() string {
	if e.Obj != nil {
		if f := e.Obj.HashTable().GetString("file"); f != nil && f.GetType() == phpv.ZtString {
			s := f.String()
			if s == "" {
				return "Unknown"
			}
			return s
		}
	}
	if e.Loc != nil {
		if e.Loc.Filename == "" {
			return "Unknown"
		}
		return e.Loc.Filename
	}
	return "Unknown"
}

// ThrownLine returns the line where the exception was constructed (from the
// exception object's "line" property), falling back to Loc.Line.
func (e *PhpThrow) ThrownLine() int {
	if e.Obj != nil {
		if l := e.Obj.HashTable().GetString("line"); l != nil && l.GetType() == phpv.ZtInt {
			return int(l.Value().(phpv.ZInt))
		}
	}
	if e.Loc != nil {
		return e.Loc.Line
	}
	return 0
}

// ErrorTrace formats the exception for uncaught-exception output.
// It calls __toString() on the exception object, which handles the full
// $previous chain. If __toString() itself throws (e.g. because the
// message property is a non-stringifiable object), the *new* error is
// returned as the fatal error string, matching PHP behaviour.
//
// The returned *PhpThrow (if non-nil) indicates that __toString() threw,
// and the caller should use this replacement exception for file/line info.
func (e *PhpThrow) ErrorTrace(ctx phpv.Context) (string, *PhpThrow) {
	toStr, err := e.Obj.ZVal().As(ctx, phpv.ZtString)
	if err != nil {
		// __toString() threw an error. PHP displays the conversion
		// error as the fatal error, with location "[no active file]:0".
		if inner, ok := phpv.UnwrapError(err).(*PhpThrow); ok {
			// Format the inner exception the normal way (recursive).
			s, replacement := inner.ErrorTrace(ctx)
			if replacement != nil {
				return s, replacement
			}
			return s, inner
		}
		// Fall back to the raw Go error text
		return "Uncaught Error: " + err.Error(), nil
	}
	s := string(toStr.Value().(phpv.ZString))
	return "Uncaught " + s, nil
}

func (e *PhpThrow) Error() string {
	if e.Obj == nil {
		return "Uncaught exception"
	}
	className := e.Obj.GetClass().GetName()
	msg := e.Obj.HashTable().GetString("message")
	if msg == nil || msg.String() == "" {
		return "Uncaught " + string(className)
	}
	return "Uncaught " + string(className) + ": " + msg.String()
}

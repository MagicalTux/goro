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

func (e *PhpThrow) ErrorTrace(ctx phpv.Context) string {
	// Use __toString() which handles the full $previous chain,
	// then prepend "Uncaught " to the first line.
	toStr := e.Obj.ZVal().AsString(ctx)
	s := string(toStr)
	return "Uncaught " + s
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

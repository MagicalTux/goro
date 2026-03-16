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
			if s := f.String(); s != "" {
				return s
			}
		}
	}
	if e.Loc != nil {
		return e.Loc.Filename
	}
	return ""
}

// ThrownLine returns the line where the exception was constructed (from the
// exception object's "line" property), falling back to Loc.Line.
func (e *PhpThrow) ThrownLine() int {
	if e.Obj != nil {
		if l := e.Obj.HashTable().GetString("line"); l != nil {
			if n, ok := l.Value().(phpv.ZInt); ok && n > 0 {
				return int(n)
			}
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
	className := e.Obj.GetClass().GetName()
	message := e.Obj.HashTable().GetString("message").String()
	if message == "" {
		return "Uncaught " + string(className)
	}
	return "Uncaught " + string(className) + ": " + message
}

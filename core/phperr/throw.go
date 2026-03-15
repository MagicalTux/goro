package phperr

import (
	"github.com/MagicalTux/goro/core/phpv"
)

type PhpThrow struct {
	Obj phpv.ZObject
	Loc *phpv.Loc
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

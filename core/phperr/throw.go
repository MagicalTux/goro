package phperr

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

type PhpThrow struct {
	Obj phpv.ZObject
	Loc *phpv.Loc
}

func (e *PhpThrow) ErrorTrace(ctx phpv.Context) string {
	className := e.Obj.GetClass().GetName()
	message := e.Obj.HashTable().GetString("message").String()
	trace := e.Obj.ZVal().AsString(ctx)
	if message == "" {
		return fmt.Sprintf(
			"Uncaught %s in %s:%d\n%s",
			className, e.Loc.Filename, e.Loc.Line, trace,
		)
	}
	return fmt.Sprintf(
		"Uncaught %s: %s in %s:%d\n%s",
		className, message, e.Loc.Filename, e.Loc.Line, trace,
	)
}

func (e *PhpThrow) Error() string {
	className := e.Obj.GetClass().GetName()
	message := e.Obj.HashTable().GetString("message").String()
	if message == "" {
		return "Uncaught " + string(className)
	}
	return "Uncaught " + string(className) + ": " + message
}

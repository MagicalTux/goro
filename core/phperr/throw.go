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
	message := e.Obj.HashTable().GetString("message").String()
	trace := e.Obj.ZVal().AsString(ctx)
	return fmt.Sprintf(
		"Fatal error: Uncaught Exception: %s in %s on line %d\n%s",
		message, e.Loc.Filename, e.Loc.Line, trace,
	)
}

func (e *PhpThrow) Error() string {
	message := e.Obj.HashTable().GetString("message").String()
	return "Uncaught Exception: " + message
}

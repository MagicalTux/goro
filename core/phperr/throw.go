package phperr

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

type PhpThrow struct {
	Obj phpv.ZObject
	Loc *phpv.Loc
}

func (e *PhpThrow) ErrorTrace(ctx phpv.Context) string {
	className := e.Obj.GetClass().GetName()
	message := e.Obj.HashTable().GetString("message").String()

	// Get the stack trace string directly (not via __toString which includes the header)
	var traceStr string
	// Walk up the class hierarchy to find the opaque trace data
	cls := e.Obj.GetClass()
	for cls != nil {
		if opaque := e.Obj.GetOpaque(cls); opaque != nil {
			if trace, ok := opaque.([]*phpv.StackTraceEntry); ok {
				traceStr = "Stack trace:\n" + string(phpv.StackTrace(trace).String())
				break
			}
		}
		cls = cls.GetParent()
	}
	if traceStr == "" {
		// Fallback: use __toString
		traceStr = string(e.Obj.ZVal().AsString(ctx))
	}

	loc := e.Loc
	if loc == nil {
		loc = &phpv.Loc{}
	}

	if message == "" {
		return fmt.Sprintf(
			"Uncaught %s in %s:%d\n%s",
			className, loc.Filename, loc.Line, traceStr,
		)
	}
	// PHP uses "and defined in" when message contains "called in" (e.g. type errors)
	locPrefix := "in"
	if strings.Contains(message, "called in") {
		locPrefix = "and defined in"
	}
	return fmt.Sprintf(
		"Uncaught %s: %s %s %s:%d\n%s",
		className, message, locPrefix, loc.Filename, loc.Line, traceStr,
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

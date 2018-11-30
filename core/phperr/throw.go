package phperr

import "github.com/MagicalTux/goro/core/phpv"

type PhpThrow struct {
	Obj phpv.ZObject
}

func (e *PhpThrow) Error() string {
	return "Uncaught Exception: ..." //TODO
}

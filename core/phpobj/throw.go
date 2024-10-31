package phpobj

import "github.com/MagicalTux/goro/core/phperr"

func Throw(o *ZObject) error {
	// TODO check if implements throwable?
	err := &phperr.PhpThrow{Obj: o}
	return err
}

// > class Throwable
var Throwable = &ZClass{
	Name: "Throwable",
	// need abstract methods:
	// getMessage getCode getFile getLine getTrace getPrevious getTraceAsString __toString
}

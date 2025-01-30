package phpobj

import (
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class Throwable
var Throwable = &ZClass{
	Name: "Throwable",
	// need abstract methods:
	// getMessage getCode getFile getLine getTrace getPrevious getTraceAsString __toString
}

func ThrowObject(ctx phpv.Context, v *phpv.ZVal) error {
	o, ok := v.Value().(*ZObject)
	if !ok {
		return ctx.Errorf("Can only throw objects")
	}
	// TODO check if implements throwable or extends Exception

	err := &phperr.PhpThrow{Obj: o, Loc: ctx.Loc()}
	return err
}

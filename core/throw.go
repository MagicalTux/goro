package core

type PhpThrow struct {
	obj *ZObject
}

func (e *PhpThrow) Error() string {
	return "Uncaught Exception: ..." //TODO
}

func Throw(o *ZObject) error {
	// TODO check if implements throwable?
	err := &PhpThrow{obj: o}
	return err
}

//> class Throwable
var Throwable = &ZClass{
	Name: "Throwable",
	// need abstract methods:
	// getMessage getCode getFile getLine getTrace getPrevious getTraceAsString __toString
}

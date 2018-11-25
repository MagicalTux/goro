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

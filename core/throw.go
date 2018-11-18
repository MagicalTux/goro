package core

type PhpThrow struct {
	obj *ZObject
}

func (e *PhpThrow) Error() string {
	return "Exception thrown"
}

func Throw(o *ZObject) error {
	// TODO check if implements throwable?
	err := &PhpThrow{obj: o}
	return err
}

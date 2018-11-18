package core

func Throw(o *ZObject) error {
	// TODO check if implements throwable?
	err := &PhpError{t: PhpThrow, obj: o}
	return err
}

package core

import "errors"

type ZClass struct {
	name ZString
}

type ZObject struct {
	h     *ZHashTable
	class *ZClass
}

func (o *ZObject) OffsetSet(key, value *ZVal) (*ZVal, error) {
	// if extending ArrayAccess → todo
	return nil, errors.New("Cannot use object of type stdClass as array")
}

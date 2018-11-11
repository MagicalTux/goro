package core

import (
	"errors"
	"fmt"
)

type ZObject struct {
	h     *ZHashTable
	class *ZClass
}

func (z *ZObject) ZVal() *ZVal {
	return &ZVal{&ZVal{z}}
}

func (z *ZObject) GetType() ZType {
	return ZtObject
}

func (z *ZObject) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtString:
		// check for __toString() method
	}

	return nil, errors.New("failed to convert object to %s")
}

func NewZObject(ctx Context, c *ZClass) (*ZObject, error) {
	n := &ZObject{h: NewHashTable(), class: c}
	// constructor call is done separately

	return n, nil
}

func (o *ZObject) OffsetSet(key, value *ZVal) (*ZVal, error) {
	// if extending ArrayAccess → todo
	return nil, errors.New("Cannot use object of type stdClass as array")
}

func (o *ZObject) CallMethod(method ZString, ctx Context, args []*ZVal) (*ZVal, error) {
	ctx = NewContext(ctx)
	ctx.SetVariable("this", o.ZVal())
	m, ok := o.class.Methods[method.ToLower()]
	if !ok {
		return nil, fmt.Errorf("Call to undefined method %s::%s()", o.class.Name, method)
	}

	return m.Method.Call(ctx, args)
}

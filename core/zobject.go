package core

import (
	"errors"
	"fmt"
)

type ZObjectAccess interface {
	ObjectGet(ctx Context, key *ZVal) (*ZVal, error)
	ObjectSet(ctx Context, key, value *ZVal) error
}

type ZObject struct {
	h     *ZHashTable
	Class *ZClass

	// for use with custom extension objects
	Opaque map[*ZClass]interface{}
}

func (z *ZObject) ZVal() *ZVal {
	return &ZVal{&ZVal{z}}
}

func (z *ZObject) GetType() ZType {
	return ZtObject
}

func (z *ZObject) GetOpaque(c *ZClass) interface{} {
	if z.Opaque == nil {
		return nil
	}
	v, ok := z.Opaque[c]
	if !ok {
		return nil
	}
	return v
}

func (z *ZObject) SetOpaque(c *ZClass, v interface{}) {
	if z.Opaque == nil {
		z.Opaque = make(map[*ZClass]interface{})
	}
	z.Opaque[c] = v
}

func (z *ZObject) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtString:
		// check for __toString() method
	}

	return nil, fmt.Errorf("failed to convert object to %s", t)
}

func NewZObject(ctx Context, c *ZClass) (*ZObject, error) {
	n := &ZObject{h: NewHashTable(), Class: c}
	return n, n.init(ctx)
}

func NewZObjectOpaque(ctx Context, c *ZClass, v interface{}) (*ZObject, error) {
	n := &ZObject{h: NewHashTable(), Class: c, Opaque: map[*ZClass]interface{}{c: v}}
	return n, n.init(ctx)
}

func (o *ZObject) init(ctx Context) error {
	// initialize object variables with default values
	for _, p := range o.Class.Props {
		if p.Default == nil {
			continue
		}
		o.h.SetString(p.VarName, p.Default.ZVal())
	}
	return nil
}

func (o *ZObject) OffsetSet(key, value *ZVal) (*ZVal, error) {
	// if extending ArrayAccess → todo
	return nil, errors.New("Cannot use object of type stdClass as array")
}

func (o *ZObject) GetMethod(method ZString, ctx Context) (Callable, error) {
	m, ok := o.Class.Methods[method.ToLower()]
	if !ok {
		m, ok = o.Class.Methods["__call"]
		if ok {
			return &callCatcher{method, m.Method}, nil
		}
		return nil, fmt.Errorf("Call to undefined method %s::%s()", o.Class.Name, method)
	}
	// TODO check method access
	return m.Method, nil
}

func (o *ZObject) ObjectGet(ctx Context, key *ZVal) (*ZVal, error) {
	var err error
	key, err = key.As(ctx, ZtString)
	if err != nil {
		return nil, err
	}

	return o.h.GetString(key.Value().(ZString)), nil
}

func (o *ZObject) ObjectSet(ctx Context, key, value *ZVal) error {
	var err error
	key, err = key.As(ctx, ZtString)
	if err != nil {
		return err
	}

	return o.h.SetString(key.Value().(ZString), value)
}

func (o *ZObject) NewIterator() ZIterator {
	return o.h.NewIterator()
}

func (a *ZObject) Count(ctx Context) ZInt {
	return a.h.count
}

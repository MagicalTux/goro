package core

import (
	"errors"
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

type ZObjectAccess interface {
	ObjectGet(ctx phpv.Context, key *phpv.ZVal) (*phpv.ZVal, error)
	ObjectSet(ctx phpv.Context, key, value *phpv.ZVal) error
}

type ZObject struct {
	h     *phpv.ZHashTable
	Class *ZClass

	// for use with custom extension objects
	Opaque map[*ZClass]interface{}
}

func (z *ZObject) ZVal() *phpv.ZVal {
	return phpv.MakeZVal(phpv.MakeZVal(z))
}

func (z *ZObject) GetType() phpv.ZType {
	return phpv.ZtObject
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

func (z *ZObject) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtString:
		// check for __toString() method
	}

	return nil, fmt.Errorf("failed to convert object to %s", t)
}

func NewZObject(ctx phpv.Context, c *ZClass) (*ZObject, error) {
	if c == nil {
		c = stdClass
	}
	n := &ZObject{h: phpv.NewHashTable(), Class: c}
	return n, n.init(ctx)
}

func NewZObjectOpaque(ctx phpv.Context, c *ZClass, v interface{}) (*ZObject, error) {
	n := &ZObject{h: phpv.NewHashTable(), Class: c, Opaque: map[*ZClass]interface{}{c: v}}
	return n, n.init(ctx)
}

func (o *ZObject) init(ctx phpv.Context) error {
	// initialize object variables with default values
	for _, p := range o.Class.Props {
		if p.Default == nil {
			continue
		}
		o.h.SetString(p.VarName, p.Default.ZVal())
	}
	return nil
}

func (o *ZObject) OffsetSet(key, value *phpv.ZVal) (*phpv.ZVal, error) {
	// if extending ArrayAccess → todo
	return nil, errors.New("Cannot use object of type stdClass as array")
}

func (o *ZObject) GetMethod(method phpv.ZString, ctx phpv.Context) (phpv.Callable, error) {
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

func (o *ZObject) ObjectGet(ctx phpv.Context, key *phpv.ZVal) (*phpv.ZVal, error) {
	var err error
	key, err = key.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	return o.h.GetString(key.Value().(phpv.ZString)), nil
}

func (o *ZObject) ObjectSet(ctx phpv.Context, key, value *phpv.ZVal) error {
	var err error
	key, err = key.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	return o.h.SetString(key.Value().(phpv.ZString), value)
}

func (o *ZObject) NewIterator() phpv.ZIterator {
	return o.h.NewIterator()
}

func (a *ZObject) Count(ctx phpv.Context) phpv.ZInt {
	return a.h.Count()
}

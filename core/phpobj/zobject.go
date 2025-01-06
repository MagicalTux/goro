package phpobj

import (
	"github.com/MagicalTux/goro/core/phpv"
)

type ZObject struct {
	h     *phpv.ZHashTable
	Class phpv.ZClass

	// for use with custom extension objects
	Opaque map[phpv.ZClass]interface{}
	ID     int
}

func (z *ZObject) ZVal() *phpv.ZVal {
	return phpv.MakeZVal(phpv.MakeZVal(z))
}

func (z *ZObject) GetType() phpv.ZType {
	return phpv.ZtObject
}

func (z *ZObject) GetOpaque(c phpv.ZClass) interface{} {
	if z.Opaque == nil {
		return nil
	}
	v, ok := z.Opaque[c]
	if !ok {
		return nil
	}
	return v
}

func (z *ZObject) SetOpaque(c phpv.ZClass, v interface{}) {
	if z.Opaque == nil {
		z.Opaque = make(map[phpv.ZClass]interface{})
	}
	z.Opaque[c] = v
}

func (z *ZObject) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtString:
		if m, ok := z.Class.GetMethod("__tostring"); ok {
			return m.Method.Call(ctx, nil)
		}
	case phpv.ZtInt:
		return phpv.ZInt(1), nil
	}

	return nil, ctx.Errorf("failed to convert object to %s", t)
}

func NewZObject(ctx phpv.Context, c phpv.ZClass) (*ZObject, error) {
	if c == nil {
		c = StdClass
	}
	n := &ZObject{h: phpv.NewHashTable(), Class: c, ID: c.NextInstanceID()}
	return n, n.init(ctx)
}

func (z *ZObject) Clone(ctx phpv.Context) (phpv.ZObject, error) {
	if len(z.Opaque) != 0 {
		// TODO allow clone callbacks
		return nil, ctx.Errorf("object cannot be cloned")
	}

	n := &ZObject{
		Class: z.Class,
		h:     z.h.Dup(), // copy on write
	}

	return n, nil
}

func NewZObjectOpaque(ctx phpv.Context, c phpv.ZClass, v interface{}) (*ZObject, error) {
	n := &ZObject{h: phpv.NewHashTable(), Class: c, Opaque: map[phpv.ZClass]interface{}{c: v}}
	return n, n.init(ctx)
}

func (o *ZObject) init(ctx phpv.Context) error {
	// initialize object variables with default values
	class := o.Class.(*ZClass)
	for _, p := range class.Props {
		if p.Default == nil {
			continue
		}
		o.h.SetString(p.VarName, p.Default.ZVal())
	}
	return nil
}

func (o *ZObject) OffsetSet(ctx phpv.Context, key, value *phpv.ZVal) (*phpv.ZVal, error) {
	// if extending ArrayAccess → todo
	return nil, ctx.Errorf("Cannot use object of type stdClass as array")
}

func (o *ZObject) GetMethod(method phpv.ZString, ctx phpv.Context) (phpv.Callable, error) {
	class := o.Class.(*ZClass)
	m, ok := class.Methods[method.ToLower()]
	if !ok {
		m, ok = class.Methods["__call"]
		if ok {
			return &callCatcher{phpv.CallableVal{}, method, m.Method}, nil
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", o.Class.GetName(), method)
	}
	// TODO check method access
	return m.Method, nil
}

func (o *ZObject) ObjectGet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	return o.h.GetString(key.(phpv.ZString)), nil
}

func (o *ZObject) ObjectSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	return o.h.SetString(key.(phpv.ZString), value)
}

func (o *ZObject) NewIterator() phpv.ZIterator {
	return o.h.NewIterator()
}

func (a *ZObject) Count(ctx phpv.Context) phpv.ZInt {
	return a.h.Count()
}

func (a *ZObject) HashTable() *phpv.ZHashTable {
	return a.h
}

func (a *ZObject) GetClass() phpv.ZClass {
	return a.Class
}

func (a *ZObject) String() string {
	return "Object"
}

func (a *ZObject) Value() phpv.Val {
	return a
}

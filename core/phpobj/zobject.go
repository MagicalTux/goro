package phpobj

import (
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/MagicalTux/goro/core/phpv"
)

type ZObject struct {
	h          *phpv.ZHashTable
	hasPrivate map[phpv.ZString]struct{}

	Class        phpv.ZClass
	CurrentClass phpv.ZClass

	// for use with custom extension objects
	Opaque map[phpv.ZClass]interface{}
	ID     int

	// Guards for __get/__set to prevent infinite recursion
	getGuard map[phpv.ZString]bool
	setGuard map[phpv.ZString]bool
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
			return ctx.CallZVal(ctx, m.Method, nil, z)
		}
	case phpv.ZtInt:
		return phpv.ZInt(1), nil
	}

	return nil, ctx.Errorf("failed to convert object to %s", t)
}

// Similar to NewZObject, but without calling the constructor
func CreateZObject(ctx phpv.Context, c phpv.ZClass) (*ZObject, error) {
	if c == nil {
		c = StdClass
	}

	n := &ZObject{
		h:          phpv.NewHashTable(),
		hasPrivate: make(map[phpv.ZString]struct{}),
		Class:      c,
		ID:         c.NextInstanceID(),
		Opaque:     map[phpv.ZClass]interface{}{},
	}

	err := n.init(ctx)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func NewZObject(ctx phpv.Context, c phpv.ZClass, args ...*phpv.ZVal) (*ZObject, error) {
	if c == nil {
		c = StdClass
	}

	// Check if class is abstract
	if zc, ok := c.(*ZClass); ok && zc.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) != 0 {
		return nil, ctx.Errorf("Cannot instantiate abstract class %s", c.GetName())
	}

	n := &ZObject{
		h:          phpv.NewHashTable(),
		hasPrivate: make(map[phpv.ZString]struct{}),
		Class:      c,
		ID:         c.NextInstanceID(),
		Opaque:     map[phpv.ZClass]interface{}{},
	}
	var constructor phpv.Callable

	err := n.init(ctx)
	if err != nil {
		return nil, err
	}

	if n.Class.Handlers() != nil && n.Class.Handlers().Constructor != nil {
		constructor = n.Class.Handlers().Constructor.Method
	} else if m, ok := n.Class.GetMethod("__construct"); ok {
		constructor = m.Method
	}

	if constructor != nil {
		_, err := ctx.CallZVal(ctx, constructor, args, n)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

func (z *ZObject) GetKin(className string) phpv.ZObject {
	class := z.Class.(*ZClass)
	for class != nil {
		if class.GetName() == phpv.ZString(className) {
			return z.new(class)
		}
		class = class.GetParent().(*ZClass)
	}
	return nil
}

func (z *ZObject) Unwrap() phpv.ZObject {
	if z != nil {
		return z.new(nil)
	}
	return z
}

func (z *ZObject) GetParent() phpv.ZObject {
	class := z.GetClass().(*ZClass)
	if z.CurrentClass != nil {
		class = z.CurrentClass.(*ZClass)
	}
	parentClass := class.GetParent().(*ZClass)
	if parentClass == nil {
		return nil
	}
	return z.new(parentClass)
}

func (z *ZObject) new(class *ZClass) *ZObject {
	return &ZObject{
		h:            z.h,
		hasPrivate:   z.hasPrivate,
		Class:        z.Class,
		CurrentClass: class,
		Opaque:       z.Opaque,
		ID:           z.ID,
	}
}

func (z *ZObject) Clone(ctx phpv.Context) (phpv.ZObject, error) {
	opaque := map[phpv.ZClass]any{}
	if len(z.Opaque) != 0 {
		for class, thing := range z.Opaque {
			if cloneable, ok := thing.(phpv.Cloneable); ok {
				thing = cloneable.Clone()
			}
			opaque[class] = thing
		}
	}

	n := &ZObject{
		Class:        z.Class,
		CurrentClass: z.CurrentClass,
		h:            z.h.Dup(), // copy on write
		hasPrivate:   maps.Clone(z.hasPrivate),
		Opaque:       opaque,
		ID:           z.Class.NextInstanceID(),
	}

	return n, nil
}

func NewZObjectOpaque(ctx phpv.Context, c phpv.ZClass, v interface{}) (*ZObject, error) {
	n := &ZObject{
		h:          phpv.NewHashTable(),
		Class:      c,
		Opaque:     map[phpv.ZClass]interface{}{c: v},
		hasPrivate: make(map[phpv.ZString]struct{}),
		ID:         c.NextInstanceID(),
	}
	return n, n.init(ctx)
}

func (o *ZObject) init(ctx phpv.Context) error {
	// initialize object variables with default values

	class := o.GetClass().(*ZClass)
	lineage := []*ZClass{}
	for class != nil {
		lineage = append(lineage, class)
		class = class.GetParent().(*ZClass)
	}

	for _, class := range slices.Backward(lineage) {
		for _, p := range class.Props {
			if p.Default == nil {
				continue
			}
			o.h.SetString(p.VarName, p.Default.ZVal())
			if p.Modifiers.IsPrivate() {
				k := getPrivatePropName(class, p.VarName)
				o.h.SetString(k, p.Default.ZVal())
				o.hasPrivate[p.VarName] = struct{}{}
			}
		}
	}

	return nil
}

func (o *ZObject) IterProps(ctx phpv.Context) iter.Seq[*phpv.ZClassProp] {
	return (&propIterator{ctx, o}).yield
}

type propIterator struct {
	ctx phpv.Context
	o   *ZObject
}

func (pi *propIterator) yield(yield func(*phpv.ZClassProp) bool) {
	o := pi.o
	ctx := pi.ctx
	shown := map[string]struct{}{}
	class := o.GetClass().(*ZClass)
Loop:
	for class != nil {
		for _, p := range class.Props {
			if !yield(p) {
				break Loop
			}
			shown[p.VarName.String()] = struct{}{}
		}
		class = class.GetParent().(*ZClass)
	}
	for k := range o.h.NewIterator().Iterate(ctx) {
		key := k.AsString(ctx)
		if _, ok := shown[string(key)]; !ok {
			p := &phpv.ZClassProp{
				VarName: key,
			}
			if !yield(p) {
				break
			}
		}
	}
}

func (o *ZObject) implementsArrayAccess() bool {
	return o.Class.Implements(ArrayAccess)
}

func (o *ZObject) callMethod(ctx phpv.Context, methodName string, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	m, err := o.GetMethod(phpv.ZString(methodName), ctx)
	if err != nil {
		return nil, err
	}
	return ctx.CallZVal(ctx, m, args, o)
}

func (o *ZObject) OffsetGet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, error) {
	if !o.implementsArrayAccess() {
		return nil, ctx.Errorf("Cannot use object of type %s as array", o.Class.GetName())
	}
	return o.callMethod(ctx, "offsetGet", key.ZVal())
}

func (o *ZObject) OffsetSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error {
	if !o.implementsArrayAccess() {
		return ctx.Errorf("Cannot use object of type %s as array", o.Class.GetName())
	}
	var keyZVal *phpv.ZVal
	if key == nil {
		keyZVal = phpv.ZNULL.ZVal()
	} else {
		keyZVal = key.ZVal()
	}
	_, err := o.callMethod(ctx, "offsetSet", keyZVal, value)
	return err
}

func (o *ZObject) OffsetExists(ctx phpv.Context, key phpv.Val) (bool, error) {
	if !o.implementsArrayAccess() {
		return false, ctx.Errorf("Cannot use object of type %s as array", o.Class.GetName())
	}
	result, err := o.callMethod(ctx, "offsetExists", key.ZVal())
	if err != nil {
		return false, err
	}
	return bool(result.AsBool(ctx)), nil
}

func (o *ZObject) OffsetUnset(ctx phpv.Context, key phpv.Val) error {
	if !o.implementsArrayAccess() {
		return ctx.Errorf("Cannot use object of type %s as array", o.Class.GetName())
	}
	_, err := o.callMethod(ctx, "offsetUnset", key.ZVal())
	return err
}

func (o *ZObject) OffsetCheck(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, bool, error) {
	exists, err := o.OffsetExists(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	val, err := o.OffsetGet(ctx, key)
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (o *ZObject) GetMethod(method phpv.ZString, ctx phpv.Context) (phpv.Callable, error) {
	class := o.GetClass().(*ZClass)
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

func (o *ZObject) HasProp(ctx phpv.Context, key phpv.Val) (bool, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return false, err
	}

	keyStr := key.(phpv.ZString)
	if _, ok := o.hasPrivate[keyStr]; ok {
		propName := getPrivatePropName(o.GetClass(), keyStr)
		if o.h.HasString(propName) {
			return true, nil
		}
	}

	if o.h.HasString(keyStr) {
		return true, nil
	}

	// Property not found, try __isset magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__isset"]; ok {
		result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
		if err != nil {
			return false, err
		}
		return bool(result.AsBool(ctx)), nil
	}

	return false, nil
}

func (o *ZObject) ObjectGet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, error) {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	keyStr := key.(phpv.ZString)
	if _, ok := o.hasPrivate[keyStr]; ok {
		propName := getPrivatePropName(o.GetClass(), keyStr)
		if o.h.HasString(propName) {
			return o.h.GetString(propName), nil
		}
	}

	if o.h.HasString(keyStr) {
		return o.h.GetString(keyStr), nil
	}

	// Property not found, try __get magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__get"]; ok {
		if o.getGuard == nil {
			o.getGuard = make(map[phpv.ZString]bool)
		}
		if !o.getGuard[keyStr] {
			o.getGuard[keyStr] = true
			result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal()}, o)
			delete(o.getGuard, keyStr)
			return result, err
		}
	}

	return o.h.GetString(keyStr), nil
}

func (o *ZObject) ObjectSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error {
	var err error
	key, err = key.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	keyStr := key.(phpv.ZString)
	if _, ok := o.hasPrivate[keyStr]; ok {
		propName := getPrivatePropName(o.GetClass(), keyStr)
		if o.h.HasString(propName) {
			return o.h.SetString(propName, value)
		}
	}

	// Check if property exists in declared props
	if o.h.HasString(keyStr) {
		return o.h.SetString(keyStr, value)
	}

	// Property not found, try __set magic method
	class := o.GetClass().(*ZClass)
	if m, ok := class.Methods["__set"]; ok {
		if o.setGuard == nil {
			o.setGuard = make(map[phpv.ZString]bool)
		}
		if !o.setGuard[keyStr] {
			o.setGuard[keyStr] = true
			_, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{keyStr.ZVal(), value}, o)
			delete(o.setGuard, keyStr)
			return err
		}
	}

	return o.h.SetString(keyStr, value)
}

func (o *ZObject) NewIterator() phpv.ZIterator {
	return o.h.NewIterator()
}

func (a *ZObject) Count(ctx phpv.Context) phpv.ZInt {
	return max(a.h.Count(), phpv.ZInt(len(a.Class.(*ZClass).Props)))
}

func (a *ZObject) HashTable() *phpv.ZHashTable {
	return a.h
}

func (a *ZObject) GetClass() phpv.ZClass {
	if c, ok := a.CurrentClass.(*ZClass); ok && c != nil {
		return a.CurrentClass
	}
	return a.Class
}

func (a *ZObject) String() string {
	return "Object"
}

func (a *ZObject) Value() phpv.Val {
	return a
}

func getPrivatePropName(class phpv.ZClass, name phpv.ZString) phpv.ZString {
	return phpv.ZString(fmt.Sprintf("*%s:%s", class.GetName(), name))
}

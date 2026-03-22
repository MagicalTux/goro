package phpobj

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

type weakRefData struct {
	obj *phpv.ZVal
}

func (d *weakRefData) Clone() any {
	return &weakRefData{obj: d.obj}
}

func getWeakRefData(o *ZObject) *weakRefData {
	d := o.GetOpaque(WeakReferenceClass)
	if d == nil {
		return nil
	}
	return d.(*weakRefData)
}

func InitWeakReference() {
	WeakReferenceClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {Name: "__construct", Modifiers: phpv.ZAttrPublic,
			Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, ThrowError(ctx, Error, "Cannot directly construct WeakReference, use WeakReference::create instead")
			})},
		"create": {Name: "create", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, ThrowError(ctx, TypeError, "WeakReference::create() expects exactly 1 argument, 0 given")
				}
				if args[0].GetType() != phpv.ZtObject {
					return nil, ThrowError(ctx, TypeError, fmt.Sprintf("WeakReference::create(): Argument #1 ($object) must be of type object, %s given", args[0].GetType().TypeName()))
				}
				ref, err := NewZObjectOpaque(ctx, WeakReferenceClass, &weakRefData{obj: args[0].Dup()})
				if err != nil {
					return nil, err
				}
				return ref.ZVal(), nil
			})},
		"get": {Name: "get", Modifiers: phpv.ZAttrPublic,
			Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getWeakRefData(o)
				if d == nil || d.obj == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.obj, nil
			})},
	}
}

// > class WeakReference
var WeakReferenceClass = &ZClass{Name: "WeakReference", Attr: phpv.ZClassFinal}

type weakMapEntry struct {
	key   *ZObject
	value *phpv.ZVal
}
type weakMapData struct {
	entries []*weakMapEntry
}

func (d *weakMapData) Clone() any {
	nd := &weakMapData{entries: make([]*weakMapEntry, len(d.entries))}
	for i, e := range d.entries {
		nd.entries[i] = &weakMapEntry{key: e.key, value: e.value.Dup()}
	}
	return nd
}
func getWeakMapData(o *ZObject) *weakMapData {
	d := o.GetOpaque(WeakMapClass)
	if d == nil {
		d = &weakMapData{}
		o.SetOpaque(WeakMapClass, d)
	}
	return d.(*weakMapData)
}
func (d *weakMapData) find(obj *ZObject) int {
	for i, e := range d.entries {
		if e.key == obj {
			return i
		}
	}
	return -1
}
func (d *weakMapData) get(obj *ZObject) (*phpv.ZVal, bool) {
	idx := d.find(obj)
	if idx < 0 {
		return nil, false
	}
	return d.entries[idx].value, true
}
func (d *weakMapData) set(obj *ZObject, val *phpv.ZVal) {
	idx := d.find(obj)
	if idx >= 0 {
		d.entries[idx].value = val
	} else {
		d.entries = append(d.entries, &weakMapEntry{key: obj, value: val})
	}
}
func (d *weakMapData) delete(obj *ZObject) {
	idx := d.find(obj)
	if idx >= 0 {
		d.entries = append(d.entries[:idx], d.entries[idx+1:]...)
	}
}

func InitWeakMap() {
	WeakMapClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {Name: "__construct", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) > 0 {
				return nil, ThrowError(ctx, Error, fmt.Sprintf("WeakMap::__construct() expects exactly 0 arguments, %d given", len(args)))
			}
			return nil, nil
		})},
		"offsetget": {Name: "offsetGet", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
				return nil, ThrowError(ctx, TypeError, "WeakMap key must be an object")
			}
			obj := args[0].Value().(*ZObject)
			d := getWeakMapData(o)
			val, ok := d.get(obj)
			if !ok {
				return nil, ThrowError(ctx, Error, fmt.Sprintf("Object %s#%d not contained in WeakMap", obj.GetClass().GetName(), obj.ID))
			}
			return val, nil
		})},
		"offsetset": {Name: "offsetSet", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 2 {
				return nil, ThrowError(ctx, TypeError, "WeakMap::offsetSet() expects 2 arguments")
			}
			if args[0].GetType() != phpv.ZtObject {
				return nil, ThrowError(ctx, TypeError, "WeakMap key must be an object")
			}
			obj := args[0].Value().(*ZObject)
			d := getWeakMapData(o)
			d.set(obj, args[1].Dup())
			return phpv.ZNULL.ZVal(), nil
		})},
		"offsetexists": {Name: "offsetExists", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
				return phpv.ZBool(false).ZVal(), nil
			}
			obj := args[0].Value().(*ZObject)
			d := getWeakMapData(o)
			_, ok := d.get(obj)
			return phpv.ZBool(ok).ZVal(), nil
		})},
		"offsetunset": {Name: "offsetUnset", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) < 1 || args[0].GetType() != phpv.ZtObject {
				return phpv.ZNULL.ZVal(), nil
			}
			obj := args[0].Value().(*ZObject)
			d := getWeakMapData(o)
			d.delete(obj)
			return phpv.ZNULL.ZVal(), nil
		})},
		"count": {Name: "count", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getWeakMapData(o)
			return phpv.ZInt(len(d.entries)).ZVal(), nil
		})},
		"getiterator": {Name: "getIterator", Modifiers: phpv.ZAttrPublic, Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getWeakMapData(o)
			arr := phpv.NewZArray()
			for _, e := range d.entries {
				entry := phpv.NewZArray()
				entry.OffsetSet(ctx, phpv.ZString("key").ZVal(), e.key.ZVal())
				entry.OffsetSet(ctx, phpv.ZString("value").ZVal(), e.value)
				arr.OffsetSet(ctx, nil, entry.ZVal())
			}
			return arr.ZVal(), nil
		})},
	}
}

// > class WeakMap
var WeakMapClass = &ZClass{Name: "WeakMap", Attr: phpv.ZClassFinal, Implementations: []*ZClass{ArrayAccess, IteratorAggregate}}

func init() {
	InitWeakReference()
	InitWeakMap()
}

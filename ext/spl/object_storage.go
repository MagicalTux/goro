package spl

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// splObjectStorageEntry holds an object and its associated info
type splObjectStorageEntry struct {
	obj  *phpobj.ZObject
	info *phpv.ZVal
}

// splObjectStorageData holds the internal state for an SplObjectStorage instance
type splObjectStorageData struct {
	// map from object hash to entry
	entries map[string]*splObjectStorageEntry
	// ordered keys for iteration
	order []string
	// current iterator position
	pos int
}

func (d *splObjectStorageData) Clone() any {
	nd := &splObjectStorageData{
		entries: make(map[string]*splObjectStorageEntry, len(d.entries)),
		order:   make([]string, len(d.order)),
		pos:     0,
	}
	copy(nd.order, d.order)
	for k, v := range d.entries {
		nd.entries[k] = &splObjectStorageEntry{
			obj:  v.obj,
			info: v.info,
		}
	}
	return nd
}

func getObjectStorageData(o *phpobj.ZObject) *splObjectStorageData {
	d := o.GetOpaque(SplObjectStorageClass)
	if d == nil {
		return nil
	}
	return d.(*splObjectStorageData)
}

func objectHash(obj *phpobj.ZObject) string {
	return fmt.Sprintf("%032x", obj.ID)
}

func initObjectStorage() {
	SplObjectStorageClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &splObjectStorageData{
					entries: make(map[string]*splObjectStorageEntry),
					order:   nil,
					pos:     0,
				}
				o.SetOpaque(SplObjectStorageClass, d)
				return nil, nil
			}),
		},
		"attach": {
			Name: "attach",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::attach(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::attach(): Argument #1 ($object) must be of type object")
				}
				info := phpv.ZNULL.ZVal()
				if len(args) > 1 && args[1] != nil {
					info = args[1]
				}
				hash := objectHash(obj)
				if _, exists := d.entries[hash]; !exists {
					d.order = append(d.order, hash)
				}
				d.entries[hash] = &splObjectStorageEntry{obj: obj, info: info}
				return nil, nil
			}),
		},
		"detach": {
			Name: "detach",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::detach(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				hash := objectHash(obj)
				if _, exists := d.entries[hash]; exists {
					delete(d.entries, hash)
					for i, h := range d.order {
						if h == hash {
							d.order = append(d.order[:i], d.order[i+1:]...)
							break
						}
					}
				}
				return nil, nil
			}),
		},
		"contains": {
			Name: "contains",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::contains(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return phpv.ZFalse.ZVal(), nil
				}
				hash := objectHash(obj)
				_, exists := d.entries[hash]
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.entries)).ZVal(), nil
			}),
		},
		"getinfo": {
			Name: "getInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if d.pos < 0 || d.pos >= len(d.order) {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "SplObjectStorage::getInfo(): Called on invalid iterator")
				}
				hash := d.order[d.pos]
				entry, exists := d.entries[hash]
				if !exists {
					return phpv.ZNULL.ZVal(), nil
				}
				return entry.info, nil
			}),
		},
		"setinfo": {
			Name: "setInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if d.pos < 0 || d.pos >= len(d.order) {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "SplObjectStorage::setInfo(): Called on invalid iterator")
				}
				info := phpv.ZNULL.ZVal()
				if len(args) > 0 && args[0] != nil {
					info = args[0]
				}
				hash := d.order[d.pos]
				if entry, exists := d.entries[hash]; exists {
					entry.info = info
				}
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				d.pos = 0
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.pos < 0 || d.pos >= len(d.order) {
					return phpv.ZFalse.ZVal(), nil
				}
				hash := d.order[d.pos]
				entry, exists := d.entries[hash]
				if !exists {
					return phpv.ZFalse.ZVal(), nil
				}
				return entry.obj.ZVal(), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.pos).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				d.pos++
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				valid := d.pos >= 0 && d.pos < len(d.order)
				return phpv.ZBool(valid).ZVal(), nil
			}),
		},
		"gethash": {
			Name: "getHash",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::getHash(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::getHash(): Argument #1 ($object) must be of type object")
				}
				return phpv.ZString(objectHash(obj)).ZVal(), nil
			}),
		},
		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::offsetExists(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return phpv.ZFalse.ZVal(), nil
				}
				hash := objectHash(obj)
				_, exists := d.entries[hash]
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::offsetGet(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return phpv.ZNULL.ZVal(), nil
				}
				hash := objectHash(obj)
				entry, exists := d.entries[hash]
				if !exists {
					return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException, fmt.Sprintf("Object not found"))
				}
				return entry.info, nil
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::offsetSet(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				info := phpv.ZNULL.ZVal()
				if len(args) > 1 && args[1] != nil {
					info = args[1]
				}
				hash := objectHash(obj)
				if _, exists := d.entries[hash]; !exists {
					d.order = append(d.order, hash)
				}
				d.entries[hash] = &splObjectStorageEntry{obj: obj, info: info}
				return nil, nil
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::offsetUnset(): Argument #1 ($object) must be of type object")
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				hash := objectHash(obj)
				if _, exists := d.entries[hash]; exists {
					delete(d.entries, hash)
					for i, h := range d.order {
						if h == hash {
							d.order = append(d.order[:i], d.order[i+1:]...)
							break
						}
					}
				}
				return nil, nil
			}),
		},
		"addall": {
			Name: "addAll",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::addAll(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				other, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::addAll(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				od := getObjectStorageData(other)
				if od == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				count := 0
				for _, hash := range od.order {
					entry := od.entries[hash]
					if _, exists := d.entries[hash]; !exists {
						d.order = append(d.order, hash)
						count++
					}
					d.entries[hash] = &splObjectStorageEntry{obj: entry.obj, info: entry.info}
				}
				return phpv.ZInt(count).ZVal(), nil
			}),
		},
		"removeall": {
			Name: "removeAll",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::removeAll(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				other, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::removeAll(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				od := getObjectStorageData(other)
				if od == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				count := 0
				for _, hash := range od.order {
					if _, exists := d.entries[hash]; exists {
						delete(d.entries, hash)
						for i, h := range d.order {
							if h == hash {
								d.order = append(d.order[:i], d.order[i+1:]...)
								break
							}
						}
						count++
					}
				}
				return phpv.ZInt(count).ZVal(), nil
			}),
		},
		"removeallexcept": {
			Name: "removeAllExcept",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::removeAllExcept(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				other, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplObjectStorage::removeAllExcept(): Argument #1 ($storage) must be of type SplObjectStorage")
				}
				od := getObjectStorageData(other)
				if od == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				count := 0
				// Build set of hashes to keep
				keep := make(map[string]bool, len(od.entries))
				for hash := range od.entries {
					keep[hash] = true
				}
				// Remove entries not in the keep set
				newOrder := make([]string, 0, len(d.order))
				for _, hash := range d.order {
					if keep[hash] {
						newOrder = append(newOrder, hash)
					} else {
						delete(d.entries, hash)
						count++
					}
				}
				d.order = newOrder
				return phpv.ZInt(count).ZVal(), nil
			}),
		},
	}
}

var SplObjectStorageClass = &phpobj.ZClass{
	Name:            "SplObjectStorage",
	Implementations: []*phpobj.ZClass{Countable, phpobj.Iterator, phpobj.ArrayAccess},
}

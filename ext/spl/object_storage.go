package spl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
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
	d := o.GetOpaqueByName("SplObjectStorage")
	if d == nil {
		return nil
	}
	return d.(*splObjectStorageData)
}

// getOrInitObjectStorageData returns the opaque data, auto-initializing if needed.
// This handles the case where a subclass constructor doesn't call parent::__construct().
func getOrInitObjectStorageData(o *phpobj.ZObject) *splObjectStorageData {
	d := getObjectStorageData(o)
	if d == nil {
		d = &splObjectStorageData{
			entries: make(map[string]*splObjectStorageEntry),
			order:   nil,
			pos:     0,
		}
		o.SetOpaque(SplObjectStorageClass, d)
	}
	return d
}

func objectHash(obj *phpobj.ZObject) string {
	return fmt.Sprintf("%032x", obj.ID)
}

func initObjectStorage() {
	SplObjectStorageClass.H = &phpv.ZClassHandlers{
		HandleCompare: func(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
			ao, aok := a.(*phpobj.ZObject)
			bo, bok := b.(*phpobj.ZObject)
			if !aok || !bok {
				return phpv.CompareUncomparable, nil
			}
			ad := getObjectStorageData(ao)
			bd := getObjectStorageData(bo)
			if ad == nil || bd == nil {
				return phpv.CompareUncomparable, nil
			}
			// Two SplObjectStorage are equal only if they contain the same objects
			// with the same associated info. A is equal to B if every object in A
			// is also in B with equal info, and vice versa.
			if len(ad.entries) != len(bd.entries) {
				return 1, nil // not equal
			}
			for hash, aEntry := range ad.entries {
				bEntry, exists := bd.entries[hash]
				if !exists {
					return 1, nil // not equal
				}
				// Compare associated info
				cmp, err := phpv.Compare(ctx, aEntry.info, bEntry.info)
				if err != nil || cmp != 0 {
					return 1, nil // not equal
				}
			}
			return 0, nil // equal
		},
	}

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
				d := getOrInitObjectStorageData(o)
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
				// Call getHash to support overridden getHash methods
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
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
				d := getOrInitObjectStorageData(o)
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
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
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
				ctx.Deprecated("Method SplObjectStorage::contains() is deprecated since 8.5, use method SplObjectStorage::offsetExists() instead", logopt.NoFuncName(true))
				d := getOrInitObjectStorageData(o)
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
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
				_, exists := d.entries[hash]
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.entries)).ZVal(), nil
			}),
		},
		"getinfo": {
			Name: "getInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.order) {
					return phpv.ZNULL.ZVal(), nil
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
				d := getOrInitObjectStorageData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.order) {
					return nil, nil
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
				d := getOrInitObjectStorageData(o)
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
				d := getOrInitObjectStorageData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.order) {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called current() on invalid iterator")
				}
				hash := d.order[d.pos]
				entry, exists := d.entries[hash]
				if !exists {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called current() on invalid iterator")
				}
				return entry.obj.ZVal(), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.pos).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
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
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				valid := d.pos >= 0 && d.pos < len(d.order)
				return phpv.ZBool(valid).ZVal(), nil
			}),
		},
		"seek": {
			Name: "seek",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Seek position 0 is out of range")
				}
				if len(args) == 0 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Seek position 0 is out of range")
				}
				pos := int(args[0].AsInt(ctx))
				if pos < 0 || pos >= len(d.order) {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, fmt.Sprintf("Seek position %d is out of range", pos))
				}
				d.pos = pos
				return nil, nil
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
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					givenType := "null"
					if len(args) > 0 && args[0] != nil {
						givenType = args[0].GetType().TypeName()
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("SplObjectStorage::offsetExists(): Argument #1 ($object) must be of type object, %s given", givenType))
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return phpv.ZFalse.ZVal(), nil
				}
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
				_, exists := d.entries[hash]
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
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
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
				entry, exists := d.entries[hash]
				if !exists {
					return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException, "Object not found")
				}
				return entry.info, nil
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					givenType := "null"
					if len(args) > 0 && args[0] != nil {
						givenType = args[0].GetType().TypeName()
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("SplObjectStorage::offsetSet(): Argument #1 ($object) must be of type object, %s given", givenType))
				}
				obj, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				info := phpv.ZNULL.ZVal()
				if len(args) > 1 && args[1] != nil {
					info = args[1]
				}
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
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
				d := getOrInitObjectStorageData(o)
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
				hash, err := callGetHash(ctx, o, obj)
				if err != nil {
					return nil, err
				}
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
				d := getOrInitObjectStorageData(o)
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
				d := getOrInitObjectStorageData(o)
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
				d := getOrInitObjectStorageData(o)
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
		"__serialize": {
			Name: "__serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				result := phpv.NewZArray()

				// Key 0: flat array of [obj, info, obj, info, ...]
				storage := phpv.NewZArray()
				if d != nil {
					idx := 0
					for _, hash := range d.order {
						entry, exists := d.entries[hash]
						if !exists {
							continue
						}
						storage.OffsetSet(ctx, phpv.ZInt(idx), entry.obj.ZVal())
						idx++
						storage.OffsetSet(ctx, phpv.ZInt(idx), entry.info)
						idx++
					}
				}
				result.OffsetSet(ctx, phpv.ZInt(0), storage.ZVal())

				// Key 1: member properties of the object
				memberProps := phpv.NewZArray()
				for prop := range o.IterProps(ctx) {
					v := o.GetPropValue(prop)
					// Use visibility-tagged key names
					var key phpv.ZString
					if prop.Modifiers.IsProtected() {
						key = phpv.ZString("\x00*\x00" + string(prop.VarName))
					} else if prop.Modifiers.IsPrivate() {
						className := o.GetClass().GetName()
						key = phpv.ZString("\x00" + string(className) + "\x00" + string(prop.VarName))
					} else {
						key = prop.VarName
					}
					memberProps.OffsetSet(ctx, key, v)
				}
				result.OffsetSet(ctx, phpv.ZInt(1), memberProps.ZVal())

				return result.ZVal(), nil
			}),
		},
		"__unserialize": {
			Name: "__unserialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				invalidErr := func(msg string) (*phpv.ZVal, error) {
					return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException, msg)
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtArray {
					return invalidErr("Incomplete or ill-typed serialization data")
				}
				arr := args[0].AsArray(ctx)
				if arr == nil || int(arr.Count(ctx)) < 2 {
					return invalidErr("Incomplete or ill-typed serialization data")
				}

				// Key 0: must be an array
				storageVal, err := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
				if err != nil || storageVal == nil || storageVal.GetType() != phpv.ZtArray {
					return invalidErr("Incomplete or ill-typed serialization data")
				}
				// Key 1: must be an array
				memberVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
				if err != nil || memberVal == nil || memberVal.GetType() != phpv.ZtArray {
					return invalidErr("Incomplete or ill-typed serialization data")
				}

				d := &splObjectStorageData{
					entries: make(map[string]*splObjectStorageEntry),
					order:   nil,
					pos:     0,
				}
				o.SetOpaque(SplObjectStorageClass, d)

				// Key 0: flat array [obj, info, obj, info, ...]
				storageArr := storageVal.AsArray(ctx)
				count := int(storageArr.Count(ctx))
				if count%2 != 0 {
					return invalidErr("Odd number of elements")
				}
				for i := 0; i < count; i += 2 {
					objVal, err1 := storageArr.OffsetGet(ctx, phpv.ZInt(i).ZVal())
					infoVal, err2 := storageArr.OffsetGet(ctx, phpv.ZInt(i+1).ZVal())
					if err1 != nil || err2 != nil || objVal == nil || objVal.GetType() != phpv.ZtObject {
						return invalidErr("Non-object key")
					}
					obj, ok := objVal.Value().(*phpobj.ZObject)
					if !ok {
						return invalidErr("Non-object key")
					}
					hash := objectHash(obj)
					if _, exists := d.entries[hash]; !exists {
						d.order = append(d.order, hash)
					}
					// Dereference the info value to strip references (PHP behavior)
					if infoVal != nil {
						infoVal = infoVal.Dup()
					}
					d.entries[hash] = &splObjectStorageEntry{obj: obj, info: infoVal}
				}

				// Key 1: member properties (already validated above)
				{
					memberArr := memberVal.AsArray(ctx)
					for k, v := range memberArr.Iterate(ctx) {
						key := string(k.AsString(ctx))
						if len(key) > 0 && key[0] == 0 {
							// Mangled name: \0*\0propName (protected) or \0ClassName\0propName (private)
							// Extract the plain property name and set it on the object
							plainName := key
							if strings.HasPrefix(key, "\x00*\x00") {
								plainName = key[3:]
							} else {
								// \0ClassName\0propName - find second \0
								idx := strings.IndexByte(key[1:], 0)
								if idx >= 0 {
									plainName = key[idx+2:]
								}
							}
							o.ObjectSet(ctx, phpv.ZString(plainName), v)
						} else {
							o.ObjectSet(ctx, k, v)
						}
					}
				}

				return nil, nil
			}),
		},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				result := phpv.NewZArray()

				// Include all properties from the object (for subclasses)
				for prop := range o.IterProps(ctx) {
					v := o.GetPropValue(prop)
					// Use visibility-tagged key names for protected/private
					var key phpv.ZString
					if prop.Modifiers.IsProtected() {
						key = phpv.ZString("\x00*\x00" + string(prop.VarName))
					} else if prop.Modifiers.IsPrivate() {
						// Get the declaring class name for private properties
						className := o.GetClass().GetName()
						key = phpv.ZString("\x00" + string(className) + "\x00" + string(prop.VarName))
					} else {
						key = prop.VarName
					}
					result.OffsetSet(ctx, key, v)
				}

				// Build the storage array with obj/inf pairs
				storage := phpv.NewZArray()
				if d != nil {
					for i, hash := range d.order {
						entry, exists := d.entries[hash]
						if !exists {
							continue
						}
						pair := phpv.NewZArray()
						pair.OffsetSet(ctx, phpv.ZString("obj"), entry.obj.ZVal())
						pair.OffsetSet(ctx, phpv.ZString("inf"), entry.info)
						storage.OffsetSet(ctx, phpv.ZInt(i), pair.ZVal())
					}
				}
				result.OffsetSet(ctx, phpv.ZString("\x00SplObjectStorage\x00storage"), storage.ZVal())
				return result.ZVal(), nil
			}),
		},
		"serialize": {
			Name: "serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitObjectStorageData(o)
				count := 0
				if d != nil {
					count = len(d.order)
				}
				serializeFn, err := ctx.Global().GetFunction(ctx, "serialize")
				if err != nil {
					return phpv.ZString("").ZVal(), nil
				}
				buf := fmt.Sprintf("x:i:%d;", count)
				if d != nil {
					for _, hash := range d.order {
						entry, exists := d.entries[hash]
						if !exists {
							continue
						}
						// Serialize object
						objSer, err := ctx.CallZVal(ctx, serializeFn, []*phpv.ZVal{entry.obj.ZVal()})
						if err != nil {
							return nil, err
						}
						// Serialize info
						infoSer, err := ctx.CallZVal(ctx, serializeFn, []*phpv.ZVal{entry.info})
						if err != nil {
							return nil, err
						}
						buf += objSer.AsString(ctx).String() + "," + infoSer.AsString(ctx).String() + ";"
					}
				}
				// Member properties
				memberProps := phpv.NewZArray()
				for prop := range o.IterProps(ctx) {
					v := o.GetPropValue(prop)
					var key phpv.ZString
					if prop.Modifiers.IsProtected() {
						key = phpv.ZString("\x00*\x00" + string(prop.VarName))
					} else if prop.Modifiers.IsPrivate() {
						className := o.GetClass().GetName()
						key = phpv.ZString("\x00" + string(className) + "\x00" + string(prop.VarName))
					} else {
						key = prop.VarName
					}
					memberProps.OffsetSet(ctx, key, v)
				}
				memberSer, err := ctx.CallZVal(ctx, serializeFn, []*phpv.ZVal{memberProps.ZVal()})
				if err != nil {
					return nil, err
				}
				buf += "m:" + memberSer.AsString(ctx).String()
				return phpv.ZString(buf).ZVal(), nil
			}),
		},
		"unserialize": {
			Name: "unserialize",
			Method: phpobj.NativeMethod(sosUnserialize),
		},
	}
}

// callGetHash calls the getHash method on the SplObjectStorage object.
// This supports overridden getHash methods in subclasses.
// We must use the real class (storage.Class) not GetClass() because the object
// may be a parent-scoped view from a parent:: call.
func callGetHash(ctx phpv.Context, storage *phpobj.ZObject, obj *phpobj.ZObject) (string, error) {
	// Look up getHash in the real class (not the CurrentClass which may be parent-scoped)
	realClass := storage.Class.(*phpobj.ZClass)
	m, ok := realClass.GetMethod("getHash")
	if !ok {
		return objectHash(obj), nil
	}
	result, err := ctx.CallZVal(ctx, m.Method, []*phpv.ZVal{obj.ZVal()}, storage)
	if err != nil {
		return "", err
	}
	if result == nil {
		return objectHash(obj), nil
	}
	// Validate return type - must be string
	if result.GetType() != phpv.ZtString {
		return "", phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s::getHash(): Return value must be of type string, %s returned",
				realClass.GetName(), result.GetType().TypeName()))
	}
	return string(result.AsString(ctx)), nil
}

// sosUnserialize implements SplObjectStorage::unserialize()
func sosUnserialize(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 || args[0] == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
			"Error at offset 0 of 0 bytes")
	}
	ser := string(args[0].AsString(ctx))
	totalLen := len(ser)
	if totalLen == 0 {
		// Empty string is a no-op (does not throw)
		return nil, nil
	}
	errAtOffset := func(offset int) error {
		return phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
			fmt.Sprintf("Error at offset %d of %d bytes", offset, totalLen))
	}
	if len(ser) < 4 || ser[:2] != "x:" {
		return nil, errAtOffset(0)
	}
	offset := 2
	if offset >= len(ser) || ser[offset] != 'i' {
		return nil, errAtOffset(offset)
	}
	offset++
	if offset >= len(ser) || ser[offset] != ':' {
		return nil, errAtOffset(offset)
	}
	offset++
	numStart := offset
	for offset < len(ser) && ser[offset] >= '0' && ser[offset] <= '9' {
		offset++
	}
	if offset == numStart || offset >= len(ser) || ser[offset] != ';' {
		return nil, errAtOffset(offset)
	}
	countStr := ser[numStart:offset]
	count, _ := strconv.Atoi(countStr)
	offset++ // skip ';'

	d := &splObjectStorageData{
		entries: make(map[string]*splObjectStorageEntry),
		pos:     0,
	}
	o.SetOpaque(SplObjectStorageClass, d)

	unserializeFn, err := ctx.Global().GetFunction(ctx, "unserialize")
	if err != nil {
		return nil, errAtOffset(offset)
	}

	// Parse each object,data; pair
	for i := 0; i < count; i++ {
		if offset >= len(ser) {
			return nil, errAtOffset(offset)
		}

		// Unserialize the object from current offset
		objResult, consumed, uErr := sosUnserializeAt(ctx, unserializeFn, ser, offset)
		if uErr != nil || objResult == nil || objResult.GetType() != phpv.ZtObject {
			return nil, errAtOffset(offset)
		}
		offset += consumed

		// Expect comma separator
		if offset >= len(ser) || ser[offset] != ',' {
			return nil, errAtOffset(offset)
		}
		offset++ // skip ','

		// Parse info value
		infoResult, consumed, uErr := sosUnserializeAt(ctx, unserializeFn, ser, offset)
		if uErr != nil {
			return nil, errAtOffset(offset)
		}
		offset += consumed

		// Expect semicolon separator
		if offset >= len(ser) || ser[offset] != ';' {
			return nil, errAtOffset(offset)
		}
		offset++ // skip ';'

		obj, ok := objResult.Value().(*phpobj.ZObject)
		if !ok {
			return nil, errAtOffset(offset)
		}

		hash := objectHash(obj)
		// Check for duplicates
		if _, exists := d.entries[hash]; exists {
			return nil, errAtOffset(offset)
		}
		d.order = append(d.order, hash)
		d.entries[hash] = &splObjectStorageEntry{obj: obj, info: infoResult}
	}

	// Parse member properties: m:serialized_array
	if offset >= len(ser) || ser[offset] != 'm' {
		return nil, errAtOffset(offset)
	}
	offset++
	if offset >= len(ser) || ser[offset] != ':' {
		return nil, errAtOffset(offset)
	}
	offset++

	memberResult, consumed, uErr := sosUnserializeAt(ctx, unserializeFn, ser, offset)
	if uErr != nil {
		return nil, errAtOffset(offset)
	}
	offset += consumed

	// Check that there's no extra data
	if offset < len(ser) {
		return nil, errAtOffset(offset)
	}

	if memberResult != nil && memberResult.GetType() == phpv.ZtArray {
		memberArr := memberResult.AsArray(ctx)
		for k, v := range memberArr.Iterate(ctx) {
			o.ObjectSet(ctx, k, v)
		}
	}

	return nil, nil
}

// sosUnserializeAt unserializes a PHP value starting at the given offset in the string.
// It returns the unserialized value, the number of bytes consumed, and any error.
func sosUnserializeAt(ctx phpv.Context, unserializeFn phpv.Callable, ser string, offset int) (*phpv.ZVal, int, error) {
	// Extract the serialized value substring from offset
	// PHP serialized values have predictable end markers, so we try to find the extent
	// We use the unserialize function on the substring and detect how many bytes were consumed
	sub := ser[offset:]

	// Try unserializing the substring
	result, err := unserializeFn.Call(ctx, []*phpv.ZVal{phpv.ZString(sub).ZVal()})
	if err != nil {
		return nil, 0, err
	}

	// Determine how many bytes were consumed by analyzing the serialized format
	consumed := serializedValueLength(sub)
	if consumed == 0 {
		return nil, 0, fmt.Errorf("could not determine serialized value length")
	}

	return result, consumed, nil
}

// serializedValueLength returns the length of the first complete serialized PHP value in s.
func serializedValueLength(s string) int {
	if len(s) == 0 {
		return 0
	}
	switch s[0] {
	case 'N': // N;
		if len(s) >= 2 && s[1] == ';' {
			return 2
		}
		return 0
	case 'b': // b:0; or b:1;
		if len(s) >= 4 && s[1] == ':' && s[3] == ';' {
			return 4
		}
		return 0
	case 'i': // i:NUM;
		if len(s) < 3 || s[1] != ':' {
			return 0
		}
		i := 2
		if i < len(s) && s[i] == '-' {
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i < len(s) && s[i] == ';' {
			return i + 1
		}
		return 0
	case 'd': // d:NUM;
		if len(s) < 3 || s[1] != ':' {
			return 0
		}
		i := 2
		if i < len(s) && s[i] == '-' {
			i++
		}
		for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.' || s[i] == 'E' || s[i] == 'e' || s[i] == '+' || s[i] == '-') {
			i++
		}
		if i < len(s) && s[i] == ';' {
			return i + 1
		}
		return 0
	case 's': // s:LEN:"...";
		if len(s) < 4 || s[1] != ':' {
			return 0
		}
		i := 2
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i >= len(s) || s[i] != ':' {
			return 0
		}
		slen, _ := strconv.Atoi(s[2:i])
		i++ // skip ':'
		if i >= len(s) || s[i] != '"' {
			return 0
		}
		i++ // skip '"'
		i += slen
		if i >= len(s) || s[i] != '"' {
			return 0
		}
		i++ // skip '"'
		if i >= len(s) || s[i] != ';' {
			return 0
		}
		return i + 1
	case 'a': // a:COUNT:{...}
		return serializedCompoundLength(s)
	case 'O': // O:LEN:"classname":COUNT:{...}
		return serializedObjectLength(s)
	case 'r', 'R': // r:NUM; or R:NUM;
		if len(s) < 3 || s[1] != ':' {
			return 0
		}
		i := 2
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i < len(s) && s[i] == ';' {
			return i + 1
		}
		return 0
	case 'C': // C:LEN:"classname":LEN:{...}
		return serializedCustomObjectLength(s)
	}
	return 0
}

// serializedCompoundLength returns the length of a serialized array a:N:{...}
func serializedCompoundLength(s string) int {
	if len(s) < 4 || s[0] != 'a' || s[1] != ':' {
		return 0
	}
	i := 2
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	count, _ := strconv.Atoi(s[2:i])
	i++ // skip ':'
	if i >= len(s) || s[i] != '{' {
		return 0
	}
	i++ // skip '{'
	for j := 0; j < count; j++ {
		// key
		kl := serializedValueLength(s[i:])
		if kl == 0 {
			return 0
		}
		i += kl
		// value
		vl := serializedValueLength(s[i:])
		if vl == 0 {
			return 0
		}
		i += vl
	}
	if i >= len(s) || s[i] != '}' {
		return 0
	}
	return i + 1
}

// serializedObjectLength returns the length of a serialized object O:LEN:"classname":COUNT:{...}
func serializedObjectLength(s string) int {
	if len(s) < 4 || s[0] != 'O' || s[1] != ':' {
		return 0
	}
	i := 2
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	nameLen, _ := strconv.Atoi(s[2:i])
	i++ // skip ':'
	if i >= len(s) || s[i] != '"' {
		return 0
	}
	i++ // skip '"'
	i += nameLen
	if i >= len(s) || s[i] != '"' {
		return 0
	}
	i++ // skip '"'
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	i++ // skip ':'
	countStart := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	propCount, _ := strconv.Atoi(s[countStart:i])
	i++ // skip ':'
	if i >= len(s) || s[i] != '{' {
		return 0
	}
	i++ // skip '{'
	for j := 0; j < propCount; j++ {
		kl := serializedValueLength(s[i:])
		if kl == 0 {
			return 0
		}
		i += kl
		vl := serializedValueLength(s[i:])
		if vl == 0 {
			return 0
		}
		i += vl
	}
	if i >= len(s) || s[i] != '}' {
		return 0
	}
	return i + 1
}

// serializedCustomObjectLength returns the length of C:LEN:"classname":LEN:{...}
func serializedCustomObjectLength(s string) int {
	if len(s) < 4 || s[0] != 'C' || s[1] != ':' {
		return 0
	}
	i := 2
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	nameLen, _ := strconv.Atoi(s[2:i])
	i++ // skip ':'
	if i >= len(s) || s[i] != '"' {
		return 0
	}
	i++ // skip '"'
	i += nameLen
	if i >= len(s) || s[i] != '"' {
		return 0
	}
	i++ // skip '"'
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	i++ // skip ':'
	dataLenStart := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return 0
	}
	dataLen, _ := strconv.Atoi(s[dataLenStart:i])
	i++ // skip ':'
	if i >= len(s) || s[i] != '{' {
		return 0
	}
	i++ // skip '{'
	i += dataLen
	if i >= len(s) || s[i] != '}' {
		return 0
	}
	return i + 1
}

var SplObjectStorageClass = &phpobj.ZClass{
	Name:            "SplObjectStorage",
	Implementations: []*phpobj.ZClass{Countable, phpobj.Iterator, phpobj.ArrayAccess, phpobj.Serializable},
}

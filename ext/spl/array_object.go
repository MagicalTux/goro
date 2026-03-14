package spl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ArrayObject constants
const (
	ArrayObjectSTD_PROP_LIST  phpv.ZInt = 1
	ArrayObjectARRAY_AS_PROPS phpv.ZInt = 2
)

// arrayObjectData holds the internal state for an ArrayObject instance
type arrayObjectData struct {
	array         *phpv.ZArray
	flags         phpv.ZInt
	iteratorClass phpv.ZString
}

func (d *arrayObjectData) Clone() any {
	return &arrayObjectData{
		array:         d.array.Dup(),
		flags:         d.flags,
		iteratorClass: d.iteratorClass,
	}
}

func getArrayObjectData(o *phpobj.ZObject) *arrayObjectData {
	d := o.GetOpaque(ArrayObjectClass)
	if d == nil {
		return nil
	}
	return d.(*arrayObjectData)
}

func initArrayObject() {
	ArrayObjectClass.Implementations = []*phpobj.ZClass{
		phpobj.IteratorAggregate,
		phpobj.ArrayAccess,
		phpobj.Serializable,
		Countable,
	}

	ArrayObjectClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"STD_PROP_LIST":  {Value: ArrayObjectSTD_PROP_LIST},
		"ARRAY_AS_PROPS": {Value: ArrayObjectARRAY_AS_PROPS},
	}

	ArrayObjectClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &arrayObjectData{
					iteratorClass: "ArrayIterator",
				}

				// Parse $array argument (default: empty array)
				if len(args) > 0 && args[0] != nil {
					switch args[0].GetType() {
					case phpv.ZtArray:
						d.array = args[0].Value().(*phpv.ZArray).Dup()
					case phpv.ZtObject:
						// Convert object to array
						arr, err := args[0].Value().AsVal(ctx, phpv.ZtArray)
						if err != nil {
							return nil, err
						}
						d.array = arr.(*phpv.ZArray).Dup()
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}

				// Parse $flags argument (default: 0)
				if len(args) > 1 && args[1] != nil {
					d.flags = args[1].AsInt(ctx)
				}

				// Parse $iteratorClass argument (default: "ArrayIterator")
				if len(args) > 2 && args[2] != nil {
					d.iteratorClass = args[2].AsString(ctx)
				}

				o.SetOpaque(ArrayObjectClass, d)
				return nil, nil
			}),
		},

		// ---- ArrayAccess methods ----

		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) < 1 {
					return phpv.ZFalse.ZVal(), nil
				}
				exists, err := d.array.OffsetExists(ctx, args[0])
				if err != nil {
					return nil, err
				}
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.array.OffsetGet(ctx, args[0])
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 2 {
					return nil, nil
				}
				key := args[0]
				value := args[1]
				// If key is null, append (like $arr[] = value)
				if key.GetType() == phpv.ZtNull {
					err := d.array.OffsetSet(ctx, nil, value)
					return nil, err
				}
				err := d.array.OffsetSet(ctx, key, value)
				return nil, err
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				err := d.array.OffsetUnset(ctx, args[0])
				return nil, err
			}),
		},

		// ---- Countable ----

		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.array.Count(ctx).ZVal(), nil
			}),
		},

		// ---- IteratorAggregate ----

		"getiterator": {
			Name: "getIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}

				// Look up the iterator class
				iterClass, err := ctx.Global().GetClass(ctx, d.iteratorClass, true)
				if err != nil {
					return nil, err
				}

				// Create the iterator with the internal array as argument
				iterObj, err := phpobj.NewZObject(ctx, iterClass, d.array.ZVal())
				if err != nil {
					return nil, err
				}

				return iterObj.ZVal(), nil
			}),
		},

		// ---- Other methods ----

		"append": {
			Name: "append",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				err := d.array.OffsetSet(ctx, nil, args[0])
				return nil, err
			}),
		},

		"getarraycopy": {
			Name: "getArrayCopy",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				return d.array.Dup().ZVal(), nil
			}),
		},

		"exchangearray": {
			Name: "exchangeArray",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				if len(args) < 1 {
					return nil, fmt.Errorf("ArrayObject::exchangeArray() expects exactly 1 argument")
				}

				oldArray := d.array

				switch args[0].GetType() {
				case phpv.ZtArray:
					d.array = args[0].Value().(*phpv.ZArray).Dup()
				case phpv.ZtObject:
					arr, err := args[0].Value().AsVal(ctx, phpv.ZtArray)
					if err != nil {
						return nil, err
					}
					d.array = arr.(*phpv.ZArray).Dup()
				default:
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("ArrayObject::exchangeArray(): Argument #1 ($array) must be of type array|object, %s given", args[0].GetType().TypeName()))
				}

				return oldArray.ZVal(), nil
			}),
		},

		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				d.flags = args[0].AsInt(ctx)
				return nil, nil
			}),
		},

		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.flags.ZVal(), nil
			}),
		},

		// ---- Sort methods ----

		"asort": {
			Name: "asort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0) // SORT_REGULAR
				if len(args) > 0 && args[0] != nil {
					sortFlags = args[0].AsInt(ctx)
				}
				arrayObjectSort(ctx, d.array, sortFlags, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"ksort": {
			Name: "ksort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0) // SORT_REGULAR
				if len(args) > 0 && args[0] != nil {
					sortFlags = args[0].AsInt(ctx)
				}
				arrayObjectSort(ctx, d.array, sortFlags, sortByKey, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"natsort": {
			Name: "natsort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				// SORT_NATURAL = 6
				arrayObjectSort(ctx, d.array, 6, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"natcasesort": {
			Name: "natcasesort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				// SORT_NATURAL | SORT_FLAG_CASE = 6 | 8 = 14
				arrayObjectSort(ctx, d.array, 6|8, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"uasort": {
			Name: "uasort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) < 1 {
					return nil, fmt.Errorf("ArrayObject::uasort() expects exactly 1 argument")
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, fmt.Errorf("ArrayObject::uasort(): Argument #1 ($callback) must be a valid callback")
				}
				err = arrayObjectUSort(ctx, d.array, cb, sortByValue)
				if err != nil {
					return nil, err
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"uksort": {
			Name: "uksort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) < 1 {
					return nil, fmt.Errorf("ArrayObject::uksort() expects exactly 1 argument")
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, fmt.Errorf("ArrayObject::uksort(): Argument #1 ($callback) must be a valid callback")
				}
				err = arrayObjectUSort(ctx, d.array, cb, sortByKey)
				if err != nil {
					return nil, err
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		// ---- Iterator class methods ----

		"setiteratorclass": {
			Name: "setIteratorClass",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				d.iteratorClass = args[0].AsString(ctx)
				return nil, nil
			}),
		},

		"getiteratorclass": {
			Name: "getIteratorClass",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZString("ArrayIterator").ZVal(), nil
				}
				return d.iteratorClass.ZVal(), nil
			}),
		},

		// ---- Serializable ----

		"serialize": {
			Name: "serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					return phpv.ZString("").ZVal(), nil
				}
				// Look up and call the global serialize() function
				fn, err := ctx.Global().GetFunction(ctx, "serialize")
				if err != nil {
					return phpv.ZString("").ZVal(), nil
				}
				result, err := ctx.CallZVal(ctx, fn, []*phpv.ZVal{d.array.ZVal()})
				if err != nil {
					return phpv.ZString("").ZVal(), nil
				}
				return result, nil
			}),
		},

		"unserialize": {
			Name: "unserialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil {
					d = &arrayObjectData{
						array:         phpv.NewZArray(),
						iteratorClass: "ArrayIterator",
					}
					o.SetOpaque(ArrayObjectClass, d)
				}
				if len(args) < 1 {
					return nil, nil
				}
				// Look up and call the global unserialize() function
				fn, err := ctx.Global().GetFunction(ctx, "unserialize")
				if err != nil {
					return nil, err
				}
				result, err := ctx.CallZVal(ctx, fn, []*phpv.ZVal{args[0]})
				if err != nil {
					return nil, err
				}
				if result != nil && result.GetType() == phpv.ZtArray {
					d.array = result.Value().(*phpv.ZArray)
				}
				return nil, nil
			}),
		},

		// ---- Magic methods for ARRAY_AS_PROPS support ----

		"__get": {
			Name:      "__get",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					return d.array.OffsetGet(ctx, args[0])
				}
				return phpv.ZNULL.ZVal(), nil
			}),
		},

		"__set": {
			Name:      "__set",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil || len(args) < 2 {
					return nil, nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					return nil, d.array.OffsetSet(ctx, args[0], args[1])
				}
				return nil, nil
			}),
		},

		"__isset": {
			Name:      "__isset",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					exists, err := d.array.OffsetExists(ctx, args[0])
					if err != nil {
						return nil, err
					}
					return phpv.ZBool(exists).ZVal(), nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},

		"__unset": {
			Name:      "__unset",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					return nil, d.array.OffsetUnset(ctx, args[0])
				}
				return nil, nil
			}),
		},
	}
}

var ArrayObjectClass = &phpobj.ZClass{
	Name: "ArrayObject",
}

// Sort helpers

type sortMode int

const (
	sortByValue sortMode = iota
	sortByKey
)

type aoSortEntry struct {
	key   *phpv.ZVal
	value *phpv.ZVal
}

func arrayObjectSort(ctx phpv.Context, array *phpv.ZArray, sortFlags phpv.ZInt, mode sortMode, reversed bool) {
	var entries []aoSortEntry
	for k, v := range array.Iterate(ctx) {
		entries = append(entries, aoSortEntry{key: k, value: v})
	}

	caseInsensitive := sortFlags&8 != 0 // SORT_FLAG_CASE
	baseFlags := sortFlags &^ 8

	sort.SliceStable(entries, func(i, j int) bool {
		var a, b *phpv.ZVal
		if mode == sortByKey {
			a, b = entries[i].key, entries[j].key
		} else {
			a, b = entries[i].value, entries[j].value
		}

		var less bool
		switch baseFlags {
		case 1: // SORT_NUMERIC
			less = a.AsInt(ctx) < b.AsInt(ctx)
		case 2: // SORT_STRING
			sa := string(a.AsString(ctx))
			sb := string(b.AsString(ctx))
			if caseInsensitive {
				sa = strings.ToUpper(sa)
				sb = strings.ToUpper(sb)
			}
			less = strings.Compare(sa, sb) < 0
		case 6: // SORT_NATURAL
			sa := string(a.AsString(ctx))
			sb := string(b.AsString(ctx))
			if caseInsensitive {
				sa = strings.ToUpper(sa)
				sb = strings.ToUpper(sb)
			}
			less = natCmp([]byte(sa), []byte(sb)) < 0
		default: // SORT_REGULAR (0)
			cmp, _ := phpv.Compare(ctx, a, b)
			less = cmp < 0
		}

		if reversed {
			return !less
		}
		return less
	})

	// Rebuild the array preserving key association
	array.Clear(ctx)
	for _, e := range entries {
		array.OffsetSet(ctx, e.key, e.value)
	}
}

func arrayObjectUSort(ctx phpv.Context, array *phpv.ZArray, compare phpv.Callable, mode sortMode) error {
	var entries []aoSortEntry
	for k, v := range array.Iterate(ctx) {
		entries = append(entries, aoSortEntry{key: k, value: v})
	}

	var sortErr error
	sort.SliceStable(entries, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		var a, b *phpv.ZVal
		if mode == sortByKey {
			a, b = entries[i].key, entries[j].key
		} else {
			a, b = entries[i].value, entries[j].value
		}
		ret, err := ctx.CallZVal(ctx, compare, []*phpv.ZVal{a, b})
		if err != nil {
			sortErr = err
			return false
		}
		return ret.AsInt(ctx) < 0
	})

	if sortErr != nil {
		return sortErr
	}

	// Rebuild preserving key association
	array.Clear(ctx)
	for _, e := range entries {
		array.OffsetSet(ctx, e.key, e.value)
	}
	return nil
}

// natCmp performs natural order string comparison (like PHP's strnatcmp).
// Returns negative if a < b, 0 if equal, positive if a > b.
func natCmp(a, b []byte) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]

		// If both are digits, compare numerically
		if isDigit(ca) && isDigit(cb) {
			// Skip leading zeros
			for ai < len(a) && a[ai] == '0' {
				ai++
			}
			for bi < len(b) && b[bi] == '0' {
				bi++
			}
			// Count digits
			na, nb := 0, 0
			for ai+na < len(a) && isDigit(a[ai+na]) {
				na++
			}
			for bi+nb < len(b) && isDigit(b[bi+nb]) {
				nb++
			}
			// More digits means bigger number
			if na != nb {
				return na - nb
			}
			// Same number of digits, compare lexicographically
			for k := 0; k < na; k++ {
				if a[ai+k] != b[bi+k] {
					return int(a[ai+k]) - int(b[bi+k])
				}
			}
			ai += na
			bi += nb
			continue
		}

		if ca != cb {
			return int(ca) - int(cb)
		}
		ai++
		bi++
	}

	return len(a) - len(b)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

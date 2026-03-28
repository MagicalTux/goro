package spl

import (
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// arrayIteratorData holds the internal state for an ArrayIterator instance
type arrayIteratorData struct {
	array        *phpv.ZArray
	iter         phpv.ZIterator
	flags        phpv.ZInt
	objectBacked bool // true when constructed from an object (not an array)
}

func (d *arrayIteratorData) Clone() any {
	return &arrayIteratorData{
		array: d.array.Dup(),
		iter:  nil, // reset iterator on clone
		flags: d.flags,
	}
}

func getArrayIteratorData(o *phpobj.ZObject) *arrayIteratorData {
	d := o.GetOpaque(ArrayIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*arrayIteratorData)
}

// getOrInitArrayIteratorData returns the opaque data, auto-initializing if needed.
func getOrInitArrayIteratorData(o *phpobj.ZObject) *arrayIteratorData {
	d := getArrayIteratorData(o)
	if d == nil {
		d = &arrayIteratorData{
			array: phpv.NewZArray(),
		}
		d.iter = d.array.NewIterator()
		o.SetOpaque(ArrayIteratorClass, d)
	}
	return d
}

// overridesMethod checks if an object's class overrides the given method
// from the specified base class. Returns true if a subclass defines its own version.
func overridesMethod(o *phpobj.ZObject, baseClass *phpobj.ZClass, methodName string) bool {
	// Use the real class (o.Class), not CurrentClass which may be a parent scope
	// when the native method is defined on the base class.
	cls, ok := o.Class.(*phpobj.ZClass)
	if !ok || cls == nil {
		return false
	}
	if cls == baseClass {
		return false // it IS the base class, not overridden
	}
	// Check if the actual class has its own definition of the method
	m, ok := cls.GetMethod(phpv.ZString(methodName))
	if !ok {
		return false
	}
	// Check if it's defined on the base class
	baseM, ok := baseClass.GetMethod(phpv.ZString(methodName))
	if !ok {
		return false
	}
	// If the method pointers differ, it's overridden
	return m != baseM
}

func initArrayIterator() {
	ArrayIteratorClass.H = &phpv.ZClassHandlers{
		HandleForeachByRef: func(ctx phpv.Context, o phpv.ZObject) (*phpv.ZArray, error) {
			if zo, ok := o.(*phpobj.ZObject); ok {
				// Subclasses that override current() cannot use foreach by reference
				if overridesMethod(zo, ArrayIteratorClass, "current") {
					return nil, nil
				}
				d := getArrayIteratorData(zo)
				if d != nil {
					return d.array, nil
				}
			}
			return nil, nil
		},
	}

	ArrayIteratorClass.Implementations = []*phpobj.ZClass{
		phpobj.Iterator,
		phpobj.ArrayAccess,
		Countable,
		phpobj.Serializable,
	}

	ArrayIteratorClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"STD_PROP_LIST":  {Value: ArrayObjectSTD_PROP_LIST},
		"ARRAY_AS_PROPS": {Value: ArrayObjectARRAY_AS_PROPS},
	}

	ArrayIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &arrayIteratorData{}
				if len(args) > 0 && args[0] != nil {
					switch args[0].GetType() {
					case phpv.ZtArray:
						d.array = args[0].Value().(*phpv.ZArray).Dup()
					case phpv.ZtObject:
						// Emit deprecation for object backing
						ctx.Deprecated("ArrayIterator::__construct(): Using an object as a backing array for ArrayIterator is deprecated, as it allows violating class constraints and invariants", logopt.NoFuncName(true))
						// Extract public properties for iteration
						obj := args[0].Value().(*phpobj.ZObject)
						d.objectBacked = true
						// Inherit flags from wrapped ArrayObject if flags not explicitly set
						if len(args) <= 1 || args[1] == nil {
							innerData := obj.GetOpaque(ArrayObjectClass)
							if innerData != nil {
								d.flags = innerData.(*arrayObjectData).flags
							}
						}
						arr := phpv.NewZArray()
						for prop := range obj.IterProps(ctx) {
							if prop.Modifiers.IsPublic() || (!prop.Modifiers.IsPrivate() && !prop.Modifiers.IsProtected()) {
								v := obj.GetPropValue(prop)
								arr.OffsetSet(ctx, prop.VarName.ZVal(), v)
							}
						}
						d.array = arr
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}
				if len(args) > 1 && args[1] != nil {
					d.flags = args[1].AsInt(ctx)
				}
				d.iter = d.array.NewIterator()
				o.SetOpaque(ArrayIteratorClass, d)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				d.iter.Reset(ctx)
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.iter.Current(ctx)
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.iter.Key(ctx)
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				d.iter.Next(ctx)
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.iter.Valid(ctx)).ZVal(), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.array.Count(ctx).ZVal(), nil
			}),
		},

		// ---- ArrayAccess methods ----

		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
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
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.array.OffsetGetWarn(ctx, args[0])
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 2 {
					return nil, nil
				}
				key := args[0]
				value := args[1]
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
				d := getOrInitArrayIteratorData(o)
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

		// ---- Flags ----

		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.flags.ZVal(), nil
			}),
		},
		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
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

		// ---- Additional methods ----

		"append": {
			Name: "append",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				// Check if backed by an object (objectStorage flag)
				if d.objectBacked {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot append properties to objects, use ArrayIterator::offsetSet() instead")
				}
				// Directly append to the array (PHP's append does not call offsetSet)
				err := d.array.OffsetSet(ctx, nil, args[0].Dup())
				return nil, err
			}),
		},

		"seek": {
			Name: "seek",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				position := int(args[0].AsInt(ctx))
				if position < 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception,
						fmt.Sprintf("Seek position %d is out of range", position))
				}
				d.iter.Reset(ctx)
				for i := 0; i < position; i++ {
					if !d.iter.Valid(ctx) {
						return nil, phpobj.ThrowError(ctx, phpobj.Exception,
							fmt.Sprintf("Seek position %d is out of range", position))
					}
					d.iter.Next(ctx)
				}
				if !d.iter.Valid(ctx) {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception,
						fmt.Sprintf("Seek position %d is out of range", position))
				}
				return nil, nil
			}),
		},

		"getarraycopy": {
			Name: "getArrayCopy",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				return d.array.Dup().ZVal(), nil
			}),
		},

		// ---- Sort methods (same as ArrayObject) ----

		"asort": {
			Name: "asort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0)
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
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0)
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
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				arrayObjectSort(ctx, d.array, 6, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"natcasesort": {
			Name: "natcasesort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				arrayObjectSort(ctx, d.array, 6|8, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"uksort": {
			Name: "uksort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) != 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayIterator::uksort() expects exactly 1 argument, %d given", len(args)))
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"ArrayIterator::uksort(): Argument #1 ($callback) must be a valid callback")
				}
				err = arrayObjectUSort(ctx, d.array, cb, sortByKey)
				if err != nil {
					return nil, err
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"uasort": {
			Name: "uasort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) != 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayIterator::uasort() expects exactly 1 argument, %d given", len(args)))
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"ArrayIterator::uasort(): Argument #1 ($callback) must be a valid callback")
				}
				err = arrayObjectUSort(ctx, d.array, cb, sortByValue)
				if err != nil {
					return nil, err
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		// ---- __debugInfo for proper var_dump/print_r output ----

		"__debuginfo": {
			Name:      "__debugInfo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}

				result := phpv.NewZArray()
				storageKey := "\x00ArrayIterator\x00storage"
				result.OffsetSet(ctx, phpv.ZString(storageKey).ZVal(), d.array.ZVal())

				return result.ZVal(), nil
			}),
		},

		// ---- Serializable ----
		"serialize": {
			Name: "serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					return phpv.ZString("").ZVal(), nil
				}
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
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					d = &arrayIteratorData{
						array: phpv.NewZArray(),
					}
					o.SetOpaque(ArrayIteratorClass, d)
				}
				if len(args) < 1 {
					return nil, nil
				}
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
					d.iter = d.array.NewIterator()
				}
				return nil, nil
			}),
		},

		// ---- __serialize / __unserialize (PHP 7.4+) ----

		"__serialize": {
			Name: "__serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayIteratorData(o)
				if d == nil {
					d = &arrayIteratorData{
						array: phpv.NewZArray(),
					}
				}

				result := phpv.NewZArray()
				result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), d.flags.ZVal())
				result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), d.array.ZVal())
				memberProps := phpv.NewZArray()
				result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), memberProps.ZVal())
				result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZNULL.ZVal())

				return result.ZVal(), nil
			}),
		},

		"__unserialize": {
			Name: "__unserialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 || args[0].GetType() != phpv.ZtArray {
					return nil, nil
				}

				arr := args[0].Value().(*phpv.ZArray)

				d := &arrayIteratorData{}

				if flagsVal, err := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal()); err == nil && flagsVal != nil {
					d.flags = flagsVal.AsInt(ctx)
				}

				if storageVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal()); err == nil && storageVal != nil {
					switch storageVal.GetType() {
					case phpv.ZtArray:
						d.array = storageVal.Value().(*phpv.ZArray)
					case phpv.ZtObject:
						ctx.Deprecated("ArrayIterator::__unserialize(): Using an object as a backing array for ArrayIterator is deprecated, as it allows violating class constraints and invariants", logopt.NoFuncName(true))
						obj := storageVal.Value().(*phpobj.ZObject)
						d.objectBacked = true
						// If the object is an ArrayObject/ArrayIterator, use its internal array
						d.array = objectStorageGetArray(ctx, obj)
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}

				d.iter = d.array.NewIterator()
				o.SetOpaque(ArrayIteratorClass, d)
				return nil, nil
			}),
		},
	}
}

var ArrayIteratorClass = &phpobj.ZClass{
	Name: "ArrayIterator",
}

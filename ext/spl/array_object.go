package spl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ArrayObject constants
const (
	ArrayObjectSTD_PROP_LIST  phpv.ZInt = 1
	ArrayObjectARRAY_AS_PROPS phpv.ZInt = 2
)

// arrayObjectData holds the internal state for an ArrayObject instance.
// When objectStorage is non-nil the ArrayObject wraps an object (deprecated in PHP 8.5);
// otherwise it stores a plain array.
type arrayObjectData struct {
	array         *phpv.ZArray
	objectStorage *phpobj.ZObject // non-nil when wrapping an object
	flags         phpv.ZInt
	iteratorClass phpv.ZString
}

func (d *arrayObjectData) Clone() any {
	cloned := &arrayObjectData{
		flags:         d.flags,
		iteratorClass: d.iteratorClass,
	}
	if d.objectStorage != nil {
		// When cloning, keep reference to the same object (PHP behavior)
		cloned.objectStorage = d.objectStorage
	} else if d.array != nil {
		cloned.array = d.array.Dup()
	}
	return cloned
}

// getStorage returns the ZVal representing the internal storage.
// If objectStorage is set, returns the object. Otherwise returns the array.
func (d *arrayObjectData) getStorage() *phpv.ZVal {
	if d.objectStorage != nil {
		return d.objectStorage.ZVal()
	}
	if d.array != nil {
		return d.array.ZVal()
	}
	return phpv.NewZArray().ZVal()
}

// getEffectiveArray returns the array to use for operations. When backed by an
// object we convert the object properties to an array on-the-fly for read ops
// that need array semantics (sorts, count, etc.).
func (d *arrayObjectData) getEffectiveArray(ctx phpv.Context) *phpv.ZArray {
	if d.objectStorage != nil {
		return objectStorageGetArray(ctx, d.objectStorage)
	}
	return d.array
}

func getArrayObjectData(o *phpobj.ZObject) *arrayObjectData {
	d := o.GetOpaque(ArrayObjectClass)
	if d == nil {
		return nil
	}
	return d.(*arrayObjectData)
}

// getOrInitArrayObjectData returns the opaque data, auto-initializing if needed.
// This handles the case where a subclass constructor doesn't call parent::__construct().
func getOrInitArrayObjectData(o *phpobj.ZObject) *arrayObjectData {
	d := getArrayObjectData(o)
	if d == nil {
		d = &arrayObjectData{
			array:         phpv.NewZArray(),
			iteratorClass: "ArrayIterator",
		}
		o.SetOpaque(ArrayObjectClass, d)
	}
	return d
}

// validateIteratorClass checks that the given class name is a valid iterator class
// (must be ArrayIterator or a subclass of it). Returns an error if invalid.
func validateIteratorClass(ctx phpv.Context, className phpv.ZString, methodName string) error {
	if className == "" || strings.EqualFold(string(className), "arrayiterator") {
		return nil
	}
	cls, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil || cls == nil {
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s: Argument #%s must be a class name derived from ArrayIterator, %s given",
				methodName, iteratorClassArgNum(methodName), className))
	}
	// Check it extends ArrayIterator (or IS ArrayIterator)
	if !cls.Implements(ArrayIteratorClass) {
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s: Argument #%s must be a class name derived from ArrayIterator, %s given",
				methodName, iteratorClassArgNum(methodName), className))
	}
	return nil
}

func iteratorClassArgNum(methodName string) string {
	if strings.Contains(methodName, "__construct") {
		return "3 ($iteratorClass)"
	}
	return "1 ($iteratorClass)"
}

// setObjectStorage sets the data to wrap an object. Emits a deprecation warning.
// The object's properties are accessed directly via its HashTable for reads/writes.
func setObjectStorage(ctx phpv.Context, d *arrayObjectData, obj *phpobj.ZObject, methodName string) error {
	ctx.Deprecated("%s: Using an object as a backing array for ArrayObject is deprecated, as it allows violating class constraints and invariants", methodName, logopt.NoFuncName(true))

	// Check for overloaded objects (like SplFixedArray) that are incompatible
	if obj.GetClass().Implements(phpobj.ArrayAccess) {
		// SplFixedArray and similar overloaded ArrayAccess objects are not compatible
		className := obj.GetClass().GetName()
		if className != "ArrayObject" && className != "ArrayIterator" {
			// Check if this class has custom ArrayAccess methods (not from ArrayObject/ArrayIterator)
			if isOverloadedArrayAccess(obj) {
				return phpobj.ThrowError(ctx, phpobj.InvalidArgumentException,
					fmt.Sprintf("Overloaded object of type %s is not compatible with ArrayObject", className))
			}
		}
	}

	d.objectStorage = obj
	d.array = nil // no separate array needed; we operate directly on the object
	return nil
}

// isOverloadedArrayAccess checks whether an object implements ArrayAccess via
// its own methods (not inherited from ArrayObject/ArrayIterator base).
func isOverloadedArrayAccess(obj *phpobj.ZObject) bool {
	cls := obj.GetClass()
	// If it IS ArrayObject or ArrayIterator, it's not "overloaded"
	if cls == ArrayObjectClass || cls == ArrayIteratorClass {
		return false
	}
	// If it extends ArrayObject or ArrayIterator, it's not "overloaded"
	if cls.Implements(ArrayObjectClass) || cls.Implements(ArrayIteratorClass) {
		return false
	}
	// Otherwise, if it implements ArrayAccess, it's considered overloaded
	return cls.Implements(phpobj.ArrayAccess)
}

// objectStorageGetArray builds a temporary array from the object's public properties
// for operations that need array semantics (like sort, count, getArrayCopy).
// For ArrayIterator/ArrayObject-backed objects, uses their internal array.
func objectStorageGetArray(ctx phpv.Context, obj *phpobj.ZObject) *phpv.ZArray {
	// Check if the object is an ArrayIterator - use its internal array
	if aiData := getArrayIteratorData(obj); aiData != nil {
		return aiData.array.Dup()
	}
	// Check if the object is an ArrayObject - use its internal array
	if aoData := getArrayObjectData(obj); aoData != nil {
		if aoData.array != nil {
			return aoData.array.Dup()
		}
	}
	arr := phpv.NewZArray()
	for prop := range obj.IterProps(ctx) {
		if prop.Modifiers.IsPublic() || (!prop.Modifiers.IsPrivate() && !prop.Modifiers.IsProtected()) {
			v := obj.GetPropValue(prop)
			arr.OffsetSet(ctx, prop.VarName.ZVal(), v)
		}
	}
	return arr
}

// getWorkingArray returns the array to operate on. For object-backed ArrayObjects,
// returns a fresh array built from the object's public properties.
func (d *arrayObjectData) getWorkingArray(ctx phpv.Context) *phpv.ZArray {
	if d.objectStorage != nil {
		return objectStorageGetArray(ctx, d.objectStorage)
	}
	return d.array
}

func initArrayObject() {
	// Set up the array cast handler for (array) support
	ArrayObjectClass.H = &phpv.ZClassHandlers{
		HandleCompare: func(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
			ao, aok := a.(*phpobj.ZObject)
			bo, bok := b.(*phpobj.ZObject)
			if !aok || !bok {
				return phpv.CompareUncomparable, nil
			}
			ad := getArrayObjectData(ao)
			bd := getArrayObjectData(bo)
			if ad == nil || bd == nil {
				return phpv.CompareUncomparable, nil
			}
			// Compare internal arrays
			aArr := ad.getEffectiveArray(ctx)
			bArr := bd.getEffectiveArray(ctx)
			if aArr == nil || bArr == nil {
				if aArr == bArr {
					return 0, nil
				}
				return 1, nil
			}
			// Also compare dynamic properties
			aDynCount := 0
			for range ao.IterProps(ctx) {
				aDynCount++
			}
			bDynCount := 0
			for range bo.IterProps(ctx) {
				bDynCount++
			}
			if aDynCount != bDynCount {
				return 1, nil
			}
			// Compare the arrays element by element
			return phpv.CompareArray(ctx, aArr, bArr)
		},
		HandleCastArray: func(ctx phpv.Context, o phpv.ZObject) (*phpv.ZArray, error) {
			if zo, ok := o.(*phpobj.ZObject); ok {
				d := zo.GetOpaque(ArrayObjectClass)
				if d != nil {
					data := d.(*arrayObjectData)
					if data.objectStorage != nil {
						return objectStorageGetArray(ctx, data.objectStorage), nil
					}
					if data.array != nil {
						return data.array.Dup(), nil
					}
				}
			}
			return phpv.NewZArray(), nil
		},
		HandleForeachByRef: func(ctx phpv.Context, o phpv.ZObject) (*phpv.ZArray, error) {
			if zo, ok := o.(*phpobj.ZObject); ok {
				d := getArrayObjectData(zo)
				if d != nil {
					if d.objectStorage != nil {
						// Cannot foreach by reference on object-backed storage
						return nil, nil
					}
					return d.array, nil
				}
			}
			return nil, nil
		},
		// Intercept property access for ARRAY_AS_PROPS support.
		// These handlers fire before __get/__set/__isset/__unset, so subclasses
		// that override those magic methods do NOT have them called when ARRAY_AS_PROPS is set.
		HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
			zo := o.(*phpobj.ZObject)
			d := getArrayObjectData(zo)
			if d == nil || d.flags&ArrayObjectARRAY_AS_PROPS == 0 {
				return nil, nil // fall through
			}
			if d.objectStorage != nil {
				v, ok := d.objectStorage.HashTable().GetStringB(key)
				if !ok {
					ctx.Warn("Undefined array key \"%s\"", key, logopt.NoFuncName(true))
					return phpv.ZNULL.ZVal(), nil
				}
				return v, nil
			}
			return d.array.OffsetGetWarn(ctx, key.ZVal())
		},
		HandlePropSet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString, value *phpv.ZVal) (bool, error) {
			zo := o.(*phpobj.ZObject)
			d := getArrayObjectData(zo)
			if d == nil || d.flags&ArrayObjectARRAY_AS_PROPS == 0 {
				return false, nil // fall through
			}
			if d.objectStorage != nil {
				return true, d.objectStorage.HashTable().SetString(key, value)
			}
			return true, d.array.OffsetSet(ctx, key.ZVal(), value)
		},
		HandlePropIsset: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (bool, bool, error) {
			zo := o.(*phpobj.ZObject)
			d := getArrayObjectData(zo)
			if d == nil || d.flags&ArrayObjectARRAY_AS_PROPS == 0 {
				return false, false, nil // fall through
			}
			if d.objectStorage != nil {
				return d.objectStorage.HashTable().HasString(key), true, nil
			}
			exists, err := d.array.OffsetExists(ctx, key.ZVal())
			if err != nil {
				return false, false, err
			}
			return exists, true, nil
		},
		HandlePropUnset: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (bool, error) {
			zo := o.(*phpobj.ZObject)
			d := getArrayObjectData(zo)
			if d == nil || d.flags&ArrayObjectARRAY_AS_PROPS == 0 {
				return false, nil // fall through
			}
			if d.objectStorage != nil {
				d.objectStorage.HashTable().UnsetString(key)
				return true, nil
			}
			return true, d.array.OffsetUnset(ctx, key.ZVal())
		},
	}

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
				// Validate arg count FIRST: at most 3
				if len(args) > 3 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayObject::__construct() expects at most 3 arguments, %d given", len(args)))
				}

				d := &arrayObjectData{
					iteratorClass: "ArrayIterator",
				}

				// Validate $iteratorClass BEFORE processing input (so no deprecation leaks on error)
				if len(args) > 2 && args[2] != nil {
					className := args[2].AsString(ctx)
					if err := validateIteratorClass(ctx, className, "ArrayObject::__construct()"); err != nil {
						return nil, err
					}
					d.iteratorClass = className
				}

				// Parse $flags argument (default: 0)
				if len(args) > 1 && args[1] != nil {
					d.flags = args[1].AsInt(ctx)
				}

				// Parse $array argument (default: empty array)
				if len(args) > 0 && args[0] != nil {
					switch args[0].GetType() {
					case phpv.ZtArray:
						d.array = args[0].Value().(*phpv.ZArray).Dup()
					case phpv.ZtObject:
						obj := args[0].Value().(*phpobj.ZObject)
						// If wrapping an ArrayObject/ArrayIterator and flags were not explicitly set,
						// inherit the flags from the wrapped object
						if len(args) <= 1 || args[1] == nil {
							innerData := obj.GetOpaque(ArrayObjectClass)
							if innerData != nil {
								d.flags = innerData.(*arrayObjectData).flags
							}
						}
						if err := setObjectStorage(ctx, d, obj, "ArrayObject::__construct()"); err != nil {
							return nil, err
						}
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}

				o.SetOpaque(ArrayObjectClass, d)
				return nil, nil
			}),
		},

		// ---- ArrayAccess methods ----

		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) < 1 {
					return phpv.ZFalse.ZVal(), nil
				}
				// Validate offset type - arrays are not valid offsets
				if args[0].GetType() == phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"Cannot access offset of type array in isset or empty")
				}
				if d.objectStorage != nil {
					// If the wrapped object implements ArrayAccess, delegate
					if d.objectStorage != o && d.objectStorage.GetClass().Implements(phpobj.ArrayAccess) {
						result, err := d.objectStorage.CallMethod(ctx, "offsetExists", args[0])
						if err != nil {
							return phpv.ZFalse.ZVal(), nil
						}
						return result, nil
					}
					// For plain objects, check the hash table directly
					key := args[0].AsString(ctx)
					return phpv.ZBool(d.objectStorage.HashTable().HasString(key)).ZVal(), nil
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
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				if args[0].GetType() == phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"Cannot access offset of type array on ArrayObject")
				}
				if d.objectStorage != nil {
					// If the wrapped object implements ArrayAccess, delegate
					if d.objectStorage != o && d.objectStorage.GetClass().Implements(phpobj.ArrayAccess) {
						return d.objectStorage.CallMethod(ctx, "offsetGet", args[0])
					}
					// For plain objects, access the hash table directly (bypasses hooks/magic)
					key := args[0].AsString(ctx)
					v, ok := d.objectStorage.HashTable().GetStringB(key)
					if !ok {
						ctx.Warn("Undefined array key \"%s\"", key, logopt.NoFuncName(true))
						return phpv.ZNULL.ZVal(), nil
					}
					return v, nil
				}
				return d.array.OffsetGetWarn(ctx, args[0])
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 2 {
					return nil, nil
				}
				key := args[0]
				value := args[1]
				if key.GetType() == phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"Cannot access offset of type array on ArrayObject")
				}
				if d.objectStorage != nil {
					// If the wrapped object implements ArrayAccess, delegate
					if d.objectStorage != o && d.objectStorage.GetClass().Implements(phpobj.ArrayAccess) {
						_, err := d.objectStorage.CallMethod(ctx, "offsetSet", key, value)
						return nil, err
					}
					// For plain objects, write directly to the hash table (bypasses magic)
					keyStr := key.AsString(ctx)
					return nil, d.objectStorage.HashTable().SetString(keyStr, value)
				}
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
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				if args[0].GetType() == phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"Cannot unset offset of type array on ArrayObject")
				}
				if d.objectStorage != nil {
					// If the wrapped object implements ArrayAccess, delegate
					if d.objectStorage != o && d.objectStorage.GetClass().Implements(phpobj.ArrayAccess) {
						_, err := d.objectStorage.CallMethod(ctx, "offsetUnset", args[0])
						return nil, err
					}
					// For plain objects, unset directly from the hash table
					keyStr := args[0].AsString(ctx)
					d.objectStorage.HashTable().UnsetString(keyStr)
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
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if d.objectStorage != nil {
					// Count the public properties on the object
					arr := objectStorageGetArray(ctx, d.objectStorage)
					return arr.Count(ctx).ZVal(), nil
				}
				return d.array.Count(ctx).ZVal(), nil
			}),
		},

		// ---- IteratorAggregate ----

		"getiterator": {
			Name: "getIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}

				// Look up the iterator class
				iterClass, err := ctx.Global().GetClass(ctx, d.iteratorClass, true)
				if err != nil {
					return nil, err
				}

				// Create the iterator
				var iterObj *phpobj.ZObject
				if d.objectStorage != nil {
					// Build a fresh array from the object's public properties
					iterArg := objectStorageGetArray(ctx, d.objectStorage).ZVal()
					var err2 error
					iterObj, err2 = phpobj.NewZObject(ctx, iterClass, iterArg)
					if err2 != nil {
						return nil, err2
					}
				} else {
					// For array-backed storage, create the iterator with a shared array reference
					var err2 error
					iterObj, err2 = phpobj.NewZObject(ctx, iterClass, d.array.ZVal())
					if err2 != nil {
						return nil, err2
					}
					// Replace the duped array with a reference to the same array
					iterData2 := getArrayIteratorData(iterObj)
					if iterData2 != nil {
						iterData2.array = d.array
						iterData2.iter = d.array.NewIterator()
					}
				}

				// Copy flags to the iterator
				iterData := getArrayIteratorData(iterObj)
				if iterData != nil {
					iterData.flags = d.flags
				}

				return iterObj.ZVal(), nil
			}),
		},

		// ---- Other methods ----

		"append": {
			Name: "append",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				if d.objectStorage != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot append properties to objects, use ArrayObject::offsetSet() instead")
				}
				// Call offsetSet(null, value) through the object so that overridden
				// offsetSet in subclasses is properly invoked.
				_, err := o.CallMethod(ctx, "offsetSet", phpv.ZNULL.ZVal(), args[0])
				return nil, err
			}),
		},

		"getarraycopy": {
			Name: "getArrayCopy",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				if d.objectStorage != nil {
					return objectStorageGetArray(ctx, d.objectStorage).ZVal(), nil
				}
				return d.array.Dup().ZVal(), nil
			}),
		},

		"exchangearray": {
			Name: "exchangeArray",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						"ArrayObject::exchangeArray() expects exactly 1 argument, 0 given")
				}

				// Save old storage for return (convert to array now, before swap)
				var oldArr *phpv.ZArray
				if d.objectStorage != nil {
					oldArr = objectStorageGetArray(ctx, d.objectStorage)
				} else if d.array != nil {
					oldArr = d.array.Dup()
				} else {
					oldArr = phpv.NewZArray()
				}

				switch args[0].GetType() {
				case phpv.ZtArray:
					d.array = args[0].Value().(*phpv.ZArray).Dup()
					d.objectStorage = nil
				case phpv.ZtObject:
					obj := args[0].Value().(*phpobj.ZObject)
					if err := setObjectStorage(ctx, d, obj, "ArrayObject::exchangeArray()"); err != nil {
						return nil, err
					}
				default:
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("ArrayObject::exchangeArray(): Argument #1 ($array) must be of type array, %s given", args[0].GetType().TypeName()))
				}

				return oldArr.ZVal(), nil
			}),
		},

		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
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
				d := getOrInitArrayObjectData(o)
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
				if ctx.Global().IsFunctionDisabled("asort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method asort when function asort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0)
				if len(args) > 0 && args[0] != nil {
					if args[0].GetType() == phpv.ZtString {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							"ArrayObject::asort(): Argument #1 ($flags) must be of type int, string given")
					}
					sortFlags = args[0].AsInt(ctx)
				}
				arrayObjectSort(ctx, d.array, sortFlags, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"ksort": {
			Name: "ksort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if ctx.Global().IsFunctionDisabled("ksort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method ksort when function ksort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				sortFlags := phpv.ZInt(0)
				if len(args) > 0 && args[0] != nil {
					if args[0].GetType() == phpv.ZtString {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							"ArrayObject::ksort(): Argument #1 ($flags) must be of type int, string given")
					}
					sortFlags = args[0].AsInt(ctx)
				}
				arrayObjectSort(ctx, d.array, sortFlags, sortByKey, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"natsort": {
			Name: "natsort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if ctx.Global().IsFunctionDisabled("natsort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method natsort when function natsort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) > 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayObject::natsort() expects exactly 0 arguments, %d given", len(args)))
				}
				arrayObjectSort(ctx, d.array, 6, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"natcasesort": {
			Name: "natcasesort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if ctx.Global().IsFunctionDisabled("natcasesort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method natcasesort when function natcasesort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) > 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayObject::natcasesort() expects exactly 0 arguments, %d given", len(args)))
				}
				arrayObjectSort(ctx, d.array, 6|8, sortByValue, false)
				return phpv.ZTrue.ZVal(), nil
			}),
		},

		"uasort": {
			Name: "uasort",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if ctx.Global().IsFunctionDisabled("uasort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method uasort when function uasort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) != 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayObject::uasort() expects exactly 1 argument, %d given", len(args)))
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"ArrayObject::uasort(): Argument #1 ($callback) must be a valid callback")
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
				if ctx.Global().IsFunctionDisabled("uksort") {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot call method uksort when function uksort is disabled")
				}
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZTrue.ZVal(), nil
				}
				if len(args) != 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("ArrayObject::uksort() expects exactly 1 argument, %d given", len(args)))
				}
				cb, err := core.SpawnCallable(ctx, args[0])
				if err != nil || cb == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"ArrayObject::uksort(): Argument #1 ($callback) must be a valid callback")
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
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) < 1 {
					return nil, nil
				}
				className := args[0].AsString(ctx)
				if err := validateIteratorClass(ctx, className, "ArrayObject::setIteratorClass()"); err != nil {
					return nil, err
				}
				d.iteratorClass = className
				return nil, nil
			}),
		},

		"getiteratorclass": {
			Name: "getIteratorClass",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.ZString("ArrayIterator").ZVal(), nil
				}
				return d.iteratorClass.ZVal(), nil
			}),
		},

		// ---- Serializable (old interface) ----

		"serialize": {
			Name: "serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
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
				d := getOrInitArrayObjectData(o)
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

		// ---- __serialize / __unserialize (PHP 7.4+) ----

		"__serialize": {
			Name: "__serialize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					d = &arrayObjectData{
						array:         phpv.NewZArray(),
						iteratorClass: "ArrayIterator",
					}
				}

				// PHP serializes ArrayObject as: {0: flags, 1: storage, 2: member_properties, 3: iterator_class_or_null}
				result := phpv.NewZArray()
				result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), d.flags.ZVal())

				// Storage: array or object (use the actual wrapped object if object-backed)
				if d.objectStorage != nil {
					result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), d.objectStorage.ZVal())
				} else if d.array != nil {
					result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), d.array.ZVal())
				} else {
					result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), phpv.NewZArray().ZVal())
				}

				// Member properties (dynamic properties on the ArrayObject itself)
				memberProps := phpv.NewZArray()
				for prop := range o.IterProps(ctx) {
					v := o.GetPropValue(prop)
					memberProps.OffsetSet(ctx, prop.VarName.ZVal(), v)
				}
				result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), memberProps.ZVal())

				// Iterator class (null if default)
				if d.iteratorClass != "" && !strings.EqualFold(string(d.iteratorClass), "arrayiterator") {
					result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), d.iteratorClass.ZVal())
				} else {
					result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZNULL.ZVal())
				}

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

				d := &arrayObjectData{
					iteratorClass: "ArrayIterator",
				}

				// Index 0: flags
				if flagsVal, err := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal()); err == nil && flagsVal != nil {
					d.flags = flagsVal.AsInt(ctx)
				}

				// Index 1: storage (array or object)
				if storageVal, err := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal()); err == nil && storageVal != nil {
					switch storageVal.GetType() {
					case phpv.ZtArray:
						d.array = storageVal.Value().(*phpv.ZArray)
					case phpv.ZtObject:
						obj := storageVal.Value().(*phpobj.ZObject)
						ctx.Deprecated("ArrayObject::__unserialize(): Using an object as a backing array for ArrayObject is deprecated, as it allows violating class constraints and invariants", logopt.NoFuncName(true))
						d.objectStorage = obj
						d.array = nil // operate directly on object
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}

				// Index 2: member properties
				if memberVal, err := arr.OffsetGet(ctx, phpv.ZInt(2).ZVal()); err == nil && memberVal != nil && memberVal.GetType() == phpv.ZtArray {
					memberArr := memberVal.Value().(*phpv.ZArray)
					for k, v := range memberArr.Iterate(ctx) {
						o.ObjectSet(ctx, k.AsString(ctx), v)
					}
				}

				// Index 3: iterator class (null means default)
				if iterVal, err := arr.OffsetGet(ctx, phpv.ZInt(3).ZVal()); err == nil && iterVal != nil && !iterVal.IsNull() {
					d.iteratorClass = iterVal.AsString(ctx)
				}

				o.SetOpaque(ArrayObjectClass, d)
				return nil, nil
			}),
		},

		// ---- __debugInfo for proper var_dump/print_r output ----

		"__debuginfo": {
			Name:      "__debugInfo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}

				result := phpv.NewZArray()

				// First include any declared/dynamic properties set on the object itself
				for prop := range o.IterProps(ctx) {
					var mangledName string
					if prop.Modifiers.IsPrivate() {
						className := string(o.GetDeclClassName(prop))
						mangledName = "\x00" + className + "\x00" + string(prop.VarName)
					} else if prop.Modifiers.IsProtected() {
						mangledName = "\x00*\x00" + string(prop.VarName)
					} else {
						mangledName = string(prop.VarName)
					}
					v := o.GetPropValue(prop)
					result.OffsetSet(ctx, phpv.ZString(mangledName).ZVal(), v)
				}

				// When wrapping self, show just the object properties
				// (skip the storage wrapper to avoid infinite recursion in var_dump)
				if d.objectStorage != nil && d.objectStorage == o {
					return result.ZVal(), nil
				}

				// Then add the internal storage as a private property
				storageKey := "\x00ArrayObject\x00storage"
				if d.objectStorage != nil {
					result.OffsetSet(ctx, phpv.ZString(storageKey).ZVal(), d.objectStorage.ZVal())
				} else if d.array != nil {
					result.OffsetSet(ctx, phpv.ZString(storageKey).ZVal(), d.array.ZVal())
				} else {
					result.OffsetSet(ctx, phpv.ZString(storageKey).ZVal(), phpv.NewZArray().ZVal())
				}

				return result.ZVal(), nil
			}),
		},

		// ---- Magic methods for ARRAY_AS_PROPS support ----

		"__get": {
			Name:      "__get",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					if d.objectStorage != nil {
						// Read from object's hash table
						key := args[0].AsString(ctx)
						v, ok := d.objectStorage.HashTable().GetStringB(key)
						if !ok {
							ctx.Warn("Undefined array key \"%s\"", key, logopt.NoFuncName(true))
							return phpv.ZNULL.ZVal(), nil
						}
						return v, nil
					}
					return d.array.OffsetGetWarn(ctx, args[0])
				}
				// When ARRAY_AS_PROPS is not set, check dynamic properties
				propName := args[0].AsString(ctx)
				v := o.HashTable().GetString(propName)
				if v != nil {
					return v, nil
				}
				ctx.Warn("Undefined property: %s::$%s", o.GetClass().GetName(), propName, logopt.NoFuncName(true))
				return phpv.ZNULL.ZVal(), nil
			}),
		},

		"__set": {
			Name:      "__set",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil || len(args) < 2 {
					return nil, nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					if d.objectStorage != nil {
						// Write directly to the object's hash table, bypassing magic methods
						keyStr := args[0].AsString(ctx)
						return nil, d.objectStorage.HashTable().SetString(keyStr, args[1])
					}
					return nil, d.array.OffsetSet(ctx, args[0], args[1])
				}
				// When ARRAY_AS_PROPS is not set, set as a dynamic property
				// on the object itself. Emit deprecation warning for PHP 8.2+.
				propName := args[0].AsString(ctx)
				// Check if this is a declared property on the class
				isDeclared := false
				if zc, ok := o.GetClass().(*phpobj.ZClass); ok {
					for cur := zc; cur != nil; cur = cur.Extends {
						for _, p := range cur.Props {
							if p.VarName == propName {
								isDeclared = true
								break
							}
						}
						if isDeclared {
							break
						}
					}
				}
				if !isDeclared && !o.AllowsDynamicProperties() {
					ctx.Deprecated("Creation of dynamic property %s::$%s is deprecated",
						o.GetClass().GetName(), propName, logopt.NoFuncName(true))
				}
				return nil, o.HashTable().SetString(propName, args[1])
			}),
		},

		"__isset": {
			Name:      "__isset",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					if d.objectStorage != nil {
						key := args[0].AsString(ctx)
						return phpv.ZBool(d.objectStorage.HashTable().HasString(key)).ZVal(), nil
					}
					exists, err := d.array.OffsetExists(ctx, args[0])
					if err != nil {
						return nil, err
					}
					return phpv.ZBool(exists).ZVal(), nil
				}
				// Check dynamic properties on the object
				propName := args[0].AsString(ctx)
				v := o.HashTable().GetString(propName)
				return phpv.ZBool(v != nil && !v.IsNull()).ZVal(), nil
			}),
		},

		"__unset": {
			Name:      "__unset",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getOrInitArrayObjectData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				if d.flags&ArrayObjectARRAY_AS_PROPS != 0 {
					if d.objectStorage != nil {
						keyStr := args[0].AsString(ctx)
						d.objectStorage.HashTable().UnsetString(keyStr)
						return nil, nil
					}
					return nil, d.array.OffsetUnset(ctx, args[0])
				}
				// Unset dynamic property
				propName := args[0].AsString(ctx)
				o.HashTable().UnsetString(propName)
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

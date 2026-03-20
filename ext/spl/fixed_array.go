package spl

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// splFixedArrayData holds the internal state for an SplFixedArray instance
type splFixedArrayData struct {
	data     []*phpv.ZVal
	pos      int
	savedPos []int // stack of saved positions for nested foreach
}

func (d *splFixedArrayData) Clone() any {
	nd := &splFixedArrayData{
		data: make([]*phpv.ZVal, len(d.data)),
		pos:  d.pos,
	}
	for i, v := range d.data {
		if v != nil {
			nd.data[i] = v.ZVal()
		}
	}
	return nd
}

// SaveIterState saves the current iterator position (for nested foreach)
func (d *splFixedArrayData) SaveIterState() {
	d.savedPos = append(d.savedPos, d.pos)
}

// RestoreIterState restores the most recently saved iterator position
func (d *splFixedArrayData) RestoreIterState() {
	if len(d.savedPos) > 0 {
		d.pos = d.savedPos[len(d.savedPos)-1]
		d.savedPos = d.savedPos[:len(d.savedPos)-1]
	}
}

func getSplFixedArrayData(o *phpobj.ZObject) *splFixedArrayData {
	d := o.GetOpaque(SplFixedArrayClass)
	if d == nil {
		return nil
	}
	return d.(*splFixedArrayData)
}

// validateIndex converts a ZVal index to an integer and validates bounds.
// Returns the index and nil error, or -1 and an error to throw.
func validateFixedArrayIndex(ctx phpv.Context, d *splFixedArrayData, indexVal *phpv.ZVal) (int, error) {
	if indexVal == nil || indexVal.IsNull() {
		return -1, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Index invalid or out of range")
	}

	// PHP SplFixedArray only accepts integer indices
	switch indexVal.GetType() {
	case phpv.ZtInt, phpv.ZtFloat, phpv.ZtBool:
		// These are OK - will be converted to int
	case phpv.ZtString:
		// Check if numeric string
		s := string(indexVal.AsString(ctx))
		isNumeric := len(s) > 0
		for i, c := range s {
			if c >= '0' && c <= '9' {
				continue
			}
			if i == 0 && (c == '-' || c == '+') && len(s) > 1 {
				continue
			}
			isNumeric = false
			break
		}
		if !isNumeric {
			return -1, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Cannot access offset of type string on SplFixedArray"))
		}
	case phpv.ZtArray:
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on SplFixedArray")
	case phpv.ZtObject:
		typeName := "object"
		if obj, ok := indexVal.Value().(*phpobj.ZObject); ok {
			typeName = string(obj.GetClass().GetName())
		}
		return -1, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot access offset of type %s on SplFixedArray", typeName))
	}

	idx := int(indexVal.AsInt(ctx))

	if idx < 0 || idx >= len(d.data) {
		return -1, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Index invalid or out of range")
	}
	return idx, nil
}

func initSplFixedArray() {
	SplFixedArrayClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				size := 0
				if len(args) > 0 && args[0] != nil && !args[0].IsNull() {
					// Type check: must be int
					switch args[0].GetType() {
					case phpv.ZtInt, phpv.ZtFloat, phpv.ZtBool:
						// Acceptable types (will be converted to int)
					case phpv.ZtNull:
						// null is deprecated in PHP 8.4+ but accepted
					default:
						typeName := args[0].GetType().TypeName()
						if args[0].GetType() == phpv.ZtObject {
							if obj, ok := args[0].Value().(*phpobj.ZObject); ok {
								typeName = string(obj.GetClass().GetName())
							}
						}
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							fmt.Sprintf("SplFixedArray::__construct(): Argument #1 ($size) must be of type int, %s given", typeName))
					}
					size = int(args[0].AsInt(ctx))
					if size < 0 || size > 1<<30 {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "SplFixedArray::__construct(): Argument #1 ($size) must be greater than or equal to 0")
					}
				}
				d := &splFixedArrayData{
					data: make([]*phpv.ZVal, size),
				}
				o.SetOpaque(SplFixedArrayClass, d)
				return nil, nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.data)).ZVal(), nil
			}),
		},
		"getsize": {
			Name: "getSize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.data)).ZVal(), nil
			}),
		},
		"setsize": {
			Name: "setSize",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 || args[0] == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "SplFixedArray::setSize(): Argument #1 ($size) must be greater than or equal to 0")
				}
				newSize := int(args[0].AsInt(ctx))
				if newSize < 0 || newSize > 1<<30 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "SplFixedArray::setSize(): Argument #1 ($size) must be greater than or equal to 0")
				}
				oldSize := len(d.data)
				if newSize == oldSize {
					return phpv.ZTrue.ZVal(), nil
				}
				if newSize < oldSize {
					d.data = d.data[:newSize]
				} else {
					nd := make([]*phpv.ZVal, newSize)
					copy(nd, d.data)
					d.data = nd
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if len(args) == 0 || args[0] == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				idx := int(args[0].AsInt(ctx))
				if idx >= 0 && idx < len(d.data) && d.data[idx] != nil {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Index invalid or out of range")
				}
				idx, err := validateFixedArrayIndex(ctx, d, args[0])
				if err != nil {
					return nil, err
				}
				if d.data[idx] == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.data[idx], nil
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Index invalid or out of range")
				}
				// null key (append) is not supported for SplFixedArray
				if args[0] == nil || args[0].IsNull() {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for SplFixedArray")
				}
				idx, err := validateFixedArrayIndex(ctx, d, args[0])
				if err != nil {
					return nil, err
				}
				d.data[idx] = args[1]
				return nil, nil
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Internal data not initialized")
				}
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, "Index invalid or out of range")
				}
				idx, err := validateFixedArrayIndex(ctx, d, args[0])
				if err != nil {
					return nil, err
				}
				d.data[idx] = nil
				return nil, nil
			}),
		},
		"toarray": {
			Name: "toArray",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				arr := phpv.NewZArray()
				for i, v := range d.data {
					if v == nil {
						arr.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
					} else {
						arr.OffsetSet(ctx, phpv.ZInt(i), v)
					}
				}
				return arr.ZVal(), nil
			}),
		},
		"__wakeup": {
			Name: "__wakeup",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.data[d.pos] == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.data[d.pos], nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZNULL.ZVal(), nil
				}
				return phpv.ZInt(d.pos).ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d != nil {
					d.pos++
				}
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d != nil {
					d.pos = 0
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil || d.pos < 0 || d.pos >= len(d.data) {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZTrue.ZVal(), nil
			}),
		},
		"fromarray": {
			Name:      "fromArray",
			Modifiers: phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "SplFixedArray::fromArray(): Argument #1 ($array) must be of type array")
				}
				arr := args[0].Value().(*phpv.ZArray)
				preserveKeys := true
				if len(args) > 1 && args[1] != nil {
					preserveKeys = bool(args[1].AsBool(ctx))
				}

				if preserveKeys {
					// Find the maximum key to determine the size
					maxKey := phpv.ZInt(-1)
					count := 0
					iter := arr.NewIterator()
					for iter.Valid(ctx) {
						k, _ := iter.Key(ctx)
						if k == nil || k.GetType() != phpv.ZtInt {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "array must contain only positive integer keys")
						}
						idx := k.Value().(phpv.ZInt)
						if idx < 0 {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "array must contain only positive integer keys")
						}
						if idx > maxKey {
							maxKey = idx
						}
						count++
						iter.Next(ctx)
					}

					size := int(maxKey) + 1
					if count == 0 {
						size = 0
					}
					if size < 0 || size > 1<<30 {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "SplFixedArray::fromArray(): array too large")
					}

					d := &splFixedArrayData{
						data: make([]*phpv.ZVal, size),
					}

					// Fill the values
					iter = arr.NewIterator()
					for iter.Valid(ctx) {
						k, _ := iter.Key(ctx)
						v, _ := iter.Current(ctx)
						idx := int(k.Value().(phpv.ZInt))
						d.data[idx] = v
						iter.Next(ctx)
					}

					obj, err := phpobj.NewZObject(ctx, SplFixedArrayClass)
					if err != nil {
						return nil, err
					}
					obj.SetOpaque(SplFixedArrayClass, d)
					return obj.ZVal(), nil
				}

				// preserveKeys = false: reindex from 0
				d := &splFixedArrayData{}
				iter := arr.NewIterator()
				for iter.Valid(ctx) {
					v, _ := iter.Current(ctx)
					d.data = append(d.data, v)
					iter.Next(ctx)
				}

				obj, err := phpobj.NewZObject(ctx, SplFixedArrayClass)
				if err != nil {
					return nil, err
				}
				obj.SetOpaque(SplFixedArrayClass, d)
				return obj.ZVal(), nil
			}),
		},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getSplFixedArrayData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				arr := phpv.NewZArray()
				for i, v := range d.data {
					if v == nil {
						arr.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
					} else {
						arr.OffsetSet(ctx, phpv.ZInt(i), v)
					}
				}
				return arr.ZVal(), nil
			}),
		},
	}
}

var SplFixedArrayClass = &phpobj.ZClass{
	Name:            "SplFixedArray",
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable, phpobj.ArrayAccess},
}


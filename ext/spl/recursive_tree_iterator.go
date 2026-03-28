package spl

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// RecursiveTreeIterator constants
const (
	rtiBypassCurrent    = 4
	rtiBypassKey        = 8
	rtiPrefixLeft       = 0
	rtiPrefixMidHasNext = 1
	rtiPrefixMidLast    = 2
	rtiPrefixEndHasNext = 3
	rtiPrefixEndLast    = 4
	rtiPrefixRight      = 5
)

type recursiveTreeIteratorData struct {
	flags   int
	prefix  [6]string
	postfix string
}

func (d *recursiveTreeIteratorData) Clone() any {
	nd := &recursiveTreeIteratorData{
		flags:   d.flags,
		prefix:  d.prefix,
		postfix: d.postfix,
	}
	return nd
}

func getRecursiveTreeIteratorData(o *phpobj.ZObject) *recursiveTreeIteratorData {
	d := o.GetOpaque(RecursiveTreeIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*recursiveTreeIteratorData)
}

var RecursiveTreeIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveTreeIterator",
	Extends:         RecursiveIteratorIteratorClass,
	Implementations: []*phpobj.ZClass{OuterIterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"BYPASS_CURRENT":      {Value: phpv.ZInt(rtiBypassCurrent)},
		"BYPASS_KEY":          {Value: phpv.ZInt(rtiBypassKey)},
		"PREFIX_LEFT":         {Value: phpv.ZInt(rtiPrefixLeft)},
		"PREFIX_MID_HAS_NEXT": {Value: phpv.ZInt(rtiPrefixMidHasNext)},
		"PREFIX_MID_LAST":     {Value: phpv.ZInt(rtiPrefixMidLast)},
		"PREFIX_END_HAS_NEXT": {Value: phpv.ZInt(rtiPrefixEndHasNext)},
		"PREFIX_END_LAST":     {Value: phpv.ZInt(rtiPrefixEndLast)},
		"PREFIX_RIGHT":        {Value: phpv.ZInt(rtiPrefixRight)},
	},
}

// treeIteratorHasNextAtDepth checks whether the iterator at the given depth
// has a next sibling (i.e. after calling next(), valid() would return true).
// This is done by checking if the parent iterator at (depth-1) has more elements
// after the current position that have children, or if the current iterator
// at `depth` itself has more elements after its current position.
// Actually, the simplest and correct approach: for each depth level in the
// stack, check if the iterator at that level has more elements after the
// current one. We do this by saving/checking the iterator state.
func treeIteratorHasNextSibling(ctx phpv.Context, d *recursiveIteratorIteratorData, depth int) bool {
	if depth < 0 || depth >= len(d.stack) {
		return false
	}
	it := d.stack[depth]

	// We need to check if after the current element at this depth,
	// there are more elements. We do this by using the CachingIterator-style
	// look-ahead. But since we don't want to modify iterator state, we use
	// a different approach: copy the iterator position, advance, check valid.
	// Actually, for RecursiveArrayIterator we can check the underlying array.

	// The simplest approach that works with any iterator: look at the
	// hasNextAtDepth tracking in the data.
	if d.hasNextAtDepth != nil && depth < len(d.hasNextAtDepth) {
		return d.hasNextAtDepth[depth]
	}
	_ = it
	return false
}

func initRecursiveTreeIterator() {
	// Start by copying all parent methods from RecursiveIteratorIterator
	RecursiveTreeIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range RecursiveIteratorIteratorClass.Methods {
		RecursiveTreeIteratorClass.Methods[k] = v
	}

	// Override specific methods
	overrides := map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::__construct() expects at least 1 argument, 0 given")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}

				flags := rtiBypassKey
				if len(args) > 1 {
					flags = int(args[1].AsInt(ctx))
				}

				// arg 2 = cachingIteratorFlags (ignored for now)
				mode := recursiveIteratorSelfFirst
				if len(args) > 3 {
					mode = int(args[3].AsInt(ctx))
				}

				// If it's an IteratorAggregate, get the real iterator
				if inner.GetClass().Implements(phpobj.IteratorAggregate) && !inner.GetClass().Implements(RecursiveIterator) {
					iterResult, err := inner.CallMethod(ctx, "getIterator")
					if err != nil {
						return nil, err
					}
					if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
						if io, ok := iterResult.Value().(*phpobj.ZObject); ok {
							inner = io
						}
					}
				}

				// Set up parent RecursiveIteratorIterator data
				d := &recursiveIteratorIteratorData{
					stack:    []*phpobj.ZObject{inner},
					mode:     mode,
					depth:    0,
					maxDepth: -1,
				}
				o.SetOpaque(RecursiveIteratorIteratorClass, d)

				// Set up tree iterator data
				td := &recursiveTreeIteratorData{
					flags: flags,
					prefix: [6]string{
						"",    // PREFIX_LEFT
						"| ",  // PREFIX_MID_HAS_NEXT
						"  ",  // PREFIX_MID_LAST
						"|-",  // PREFIX_END_HAS_NEXT
						"\\-", // PREFIX_END_LAST
						"",    // PREFIX_RIGHT
					},
				}
				o.SetOpaque(RecursiveTreeIteratorClass, td)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object is not initialized")
				}
				if len(d.stack) == 0 {
					return nil, nil
				}
				// Reset to just the root iterator
				root := d.stack[0]
				d.stack = []*phpobj.ZObject{root}
				d.depth = 0
				d.hasNextAtDepth = nil
				_, err := root.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				// Call beginIteration hook
				o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "beginIteration")
				// Descend into children if needed
				err = recursiveIteratorDescend(ctx, d, o)
				if err != nil {
					return nil, err
				}
				// Update hasNext tracking
				treeIteratorUpdateAllHasNext(ctx, d)
				// Call nextElement hook after first element is ready
				v, _ := o.CallMethod(ctx, "valid")
				if v != nil && bool(v.AsBool(ctx)) {
					_, err = o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "nextElement")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return nil, nil
				}
				err := recursiveIteratorNext(ctx, d, o)
				if err != nil {
					return nil, err
				}
				// Update hasNext tracking
				treeIteratorUpdateAllHasNext(ctx, d)
				// Call nextElement hook after next element is ready
				v, _ := o.CallMethod(ctx, "valid")
				if v != nil && bool(v.AsBool(ctx)) {
					_, err = o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "nextElement")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"getprefix": {
			Name: "getPrefix",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return phpv.ZStr(""), nil
				}
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return phpv.ZStr(""), nil
				}

				prefix := td.prefix[rtiPrefixLeft]
				depth := len(d.stack) - 1

				// For each intermediate level, show whether there's a next sibling
				for i := 0; i < depth; i++ {
					hasNext := false
					if d.hasNextAtDepth != nil && i < len(d.hasNextAtDepth) {
						hasNext = d.hasNextAtDepth[i]
					}
					if hasNext {
						prefix += td.prefix[rtiPrefixMidHasNext]
					} else {
						prefix += td.prefix[rtiPrefixMidLast]
					}
				}
				// For the current depth level
				if depth >= 0 {
					hasNext := false
					if d.hasNextAtDepth != nil && depth < len(d.hasNextAtDepth) {
						hasNext = d.hasNextAtDepth[depth]
					}
					if hasNext {
						prefix += td.prefix[rtiPrefixEndHasNext]
					} else {
						prefix += td.prefix[rtiPrefixEndLast]
					}
				}
				prefix += td.prefix[rtiPrefixRight]
				return phpv.ZStr(prefix), nil
			}),
		},
		"setprefixpart": {
			Name: "setPrefixPart",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return nil, nil
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::setPrefixPart() expects exactly 2 arguments")
				}
				part := int(args[0].AsInt(ctx))
				if part < 0 || part > 5 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
						"RecursiveTreeIterator::setPrefixPart(): Argument #1 ($part) must be a RecursiveTreeIterator::PREFIX_* constant")
				}
				td.prefix[part] = string(args[1].AsString(ctx))
				return nil, nil
			}),
		},
		"getentry": {
			Name: "getEntry",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZStr(""), nil
				}
				top := d.stack[len(d.stack)-1]
				v, err := top.CallMethod(ctx, "current")
				if err != nil {
					return phpv.ZStr(""), nil
				}
				// Convert to string without triggering "Array to string" warning
				// (PHP's internal getEntry uses a quiet conversion)
				if v.GetType() == phpv.ZtArray {
					return phpv.ZStr("Array"), nil
				}
				return v.AsString(ctx).ZVal(), nil
			}),
		},
		"getpostfix": {
			Name: "getPostfix",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return phpv.ZStr(""), nil
				}
				return phpv.ZStr(td.postfix), nil
			}),
		},
		"setpostfix": {
			Name: "setPostfix",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return nil, nil
				}
				if len(args) > 0 {
					td.postfix = string(args[0].AsString(ctx))
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}

				if td != nil && td.flags&rtiBypassCurrent != 0 {
					top := d.stack[len(d.stack)-1]
					return top.CallMethod(ctx, "current")
				}

				// Build prefix + entry + postfix
				prefix, _ := o.CallMethod(ctx, "getPrefix")
				entry, _ := o.CallMethod(ctx, "getEntry")
				postfix, _ := o.CallMethod(ctx, "getPostfix")

				result := string(prefix.AsString(ctx)) + string(entry.AsString(ctx)) + string(postfix.AsString(ctx))
				return phpv.ZStr(result), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}

				top := d.stack[len(d.stack)-1]
				key, err := top.CallMethod(ctx, "key")
				if err != nil {
					return phpv.ZNULL.ZVal(), nil
				}

				if td != nil && td.flags&rtiBypassKey != 0 {
					return key, nil
				}

				// Prepend prefix to key
				prefix, _ := o.CallMethod(ctx, "getPrefix")
				return phpv.ZStr(string(prefix.AsString(ctx)) + string(key.AsString(ctx))), nil
			}),
		},
	}
	for k, v := range overrides {
		RecursiveTreeIteratorClass.Methods[k] = v
	}
}

// treeIteratorUpdateAllHasNext updates the hasNextAtDepth array for the current
// position. For each depth level, it checks whether the iterator at that level
// has more elements after the current one by looking ahead.
func treeIteratorUpdateAllHasNext(ctx phpv.Context, d *recursiveIteratorIteratorData) {
	depth := len(d.stack) - 1
	if depth < 0 {
		return
	}
	// Grow slice if needed
	for len(d.hasNextAtDepth) <= depth {
		d.hasNextAtDepth = append(d.hasNextAtDepth, false)
	}
	// Trim if stack is smaller
	if len(d.hasNextAtDepth) > depth+1 {
		d.hasNextAtDepth = d.hasNextAtDepth[:depth+1]
	}

	// For each depth level, check if the iterator has a next sibling.
	// We need to "peek ahead" without modifying the iterator state.
	// For RecursiveArrayIterator, we can check the underlying array.
	// For other iterators, we need a different approach.
	for i := 0; i <= depth; i++ {
		d.hasNextAtDepth[i] = iteratorHasNextElement(ctx, d.stack[i])
	}
}

// iteratorHasNextElement checks if the given iterator has more elements after
// the current one. It does this by calling next(), checking valid(), then
// rewinding back. However, this is destructive for non-seekable iterators.
// For RecursiveArrayIterator, we can use a safer approach.
func iteratorHasNextElement(ctx phpv.Context, it *phpobj.ZObject) bool {
	// Try to get current key and count for array-based iterators
	// Check if this is a RecursiveArrayIterator by checking for getArrayCopy method
	d := getRecursiveArrayIteratorData(it)
	if d != nil {
		// We have direct access to the array, check if there are more elements
		// after the current position
		if !d.iter.Valid(ctx) {
			return false
		}
		// Save current position, advance, check, restore
		// Use a simpler approach: count remaining elements
		curKey, _ := d.iter.Key(ctx)
		if curKey == nil {
			return false
		}

		// Iterate forward to count remaining
		savedIter := d.array.NewIterator()
		// Seek to current position
		for savedIter.Valid(ctx) {
			k, _ := savedIter.Key(ctx)
			if k != nil && k.String() == curKey.String() {
				savedIter.Next(ctx)
				return savedIter.Valid(ctx)
			}
			savedIter.Next(ctx)
		}
		return false
	}

	// For other iterators (like RecursiveCallbackFilterIterator etc),
	// we cannot peek without modifying state, so default to false.
	// This is a limitation but works for the common case.
	return false
}

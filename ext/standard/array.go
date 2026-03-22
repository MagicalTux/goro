package standard

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	CASE_LOWER phpv.ZInt = 0
	CASE_UPPER phpv.ZInt = 1
)

// > const
const (
	ARRAY_FILTER_USE_BOTH phpv.ZInt = 1
	ARRAY_FILTER_USE_KEY  phpv.ZInt = 2
)

// > const
const (
	SORT_REGULAR phpv.ZInt = iota
	SORT_NUMERIC
	SORT_STRING
	SORT_DESC
	SORT_ASC
	SORT_LOCALE_STRING
	SORT_NATURAL

	SORT_FLAG_CASE phpv.ZInt = 8
)

// > const
const (
	EXTR_OVERWRITE phpv.ZInt = iota
	EXTR_SKIP
	EXTR_PREFIX_SAME
	EXTR_PREFIX_ALL
	EXTR_PREFIX_INVALID
	EXTR_PREFIX_IF_EXISTS
	EXTR_IF_EXISTS

	EXTR_REFS phpv.ZInt = 0x100
)

// > func array array_combine ( array $keys , array $values )
func fncArrayCombine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var keys, values *phpv.ZArray
	_, err := core.Expand(ctx, args, &keys, &values)
	if err != nil {
		return nil, err
	}

	if keys.Count(ctx) != values.Count(ctx) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_combine(): Argument #1 ($keys) and argument #2 ($values) must have the same number of elements")
	}

	result := phpv.NewZArray()
	keyIter := keys.NewIterator()
	valIter := values.NewIterator()

	for keyIter.Valid(ctx) && valIter.Valid(ctx) {
		key, err := keyIter.Current(ctx)
		if err != nil {
			return nil, err
		}
		val, err := valIter.Current(ctx)
		if err != nil {
			return nil, err
		}
		// array_combine converts key values to string first, then applies
		// standard array key coercion. This means float 1.1 becomes string
		// key "1.1" (not truncated to int 1).
		keyStr, err := key.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		err = result.OffsetSet(ctx, keyStr, val)
		if err != nil {
			return nil, err
		}

		_, err = keyIter.Next(ctx)
		if err != nil {
			return nil, err
		}
		_, err = valIter.Next(ctx)
		if err != nil {
			return nil, err
		}
	}

	return result.ZVal(), nil
}

// > func array array_merge ( array $array1 [, array $... ] )
const arrayMergeMaxElements = 1<<31 - 1 // ~2 billion, matches PHP's HT_MAX_SIZE

func fncArrayMerge(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate all arguments are arrays first
	for i, arg := range args {
		if arg.GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("array_merge(): Argument #%d must be of type array, %s given", i+1, arg.GetType().TypeName()))
		}
	}

	var a *phpv.ZArray
	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	// Pre-check total element count to prevent OOM from massive merges
	totalCount := int64(a.Count(ctx))
	for i := 1; i < len(args); i++ {
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		totalCount += int64(b.Value().(*phpv.ZArray).Count(ctx))
		if totalCount > arrayMergeMaxElements {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("The total number of elements must be lower than %d", arrayMergeMaxElements+1))
		}
	}

	a = a.Dup() // make sure we do a copy of array

	for i := 1; i < len(args); i++ {
		// Check deadline to prevent hanging on massive merges
		if err := ctx.Tick(ctx, nil); err != nil {
			return nil, err
		}
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		err = a.MergeTable(b.HashTable())
		if err != nil {
			return nil, err
		}
	}

	a.Reset(ctx)

	return a.ZVal(), nil
}

// > func array array_replace ( array $array1 [, array $... ] )
func fncArrayReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, err
	}
	result := array.Dup()

	for i := 1; i < len(args); i++ {
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		for k, v := range b.AsArray(ctx).Iterate(ctx) {
			result.OffsetSet(ctx, k, v)
		}

	}

	return result.ZVal(), nil
}

// > func bool in_array ( mixed $needle , array $haystack [, bool $strict = FALSE ] )
func fncInArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var needle *phpv.ZVal
	var haystack *phpv.ZArray
	var strictArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &needle, &haystack, &strictArg)
	if err != nil {
		return nil, err
	}

	strict := false
	if strictArg != nil {
		strict = bool(*strictArg)
	}

	iter := haystack.NewIterator()

	for ; iter.Valid(ctx); iter.Next(ctx) {
		val, err := iter.Current(ctx)
		if err != nil {
			return nil, err
		}

		if strict {
			eq, err := phpv.StrictEquals(ctx, needle, val)
			if err != nil {
				return nil, err
			}
			if eq {
				return phpv.ZBool(true).ZVal(), nil
			}
		} else {
			eq, err := phpv.Equals(ctx, needle, val)
			if err != nil {
				return nil, err
			}
			if eq {
				return phpv.ZBool(true).ZVal(), nil
			}
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

// > func bool array_key_exists (  mixed $key , array $array )
// > alias key_exists
func fncArrayKeyExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var key *phpv.ZVal
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &key, &array)
	if err != nil {
		return nil, err
	}

	if _, ok := key.Value().(phpv.ZNull); ok {
		if err := ctx.Deprecated("Using null as the key parameter for array_key_exists() is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
		// Convert null to empty string for the lookup
		key = phpv.ZString("").ZVal()
	}

	exists, err := array.OffsetExists(ctx, key)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(exists).ZVal(), nil

}

// > func array array_values ( array $array )
func fncArrayValues(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, err
	}

	result := phpv.NewZArray()
	iter := array.NewIterator()

	for ; iter.Valid(ctx); iter.Next(ctx) {
		val, err := iter.Current(ctx)
		if err != nil {
			return nil, err
		}

		result.OffsetSet(ctx, nil, val)
	}

	return result.ZVal(), nil
}

// > func array array_keys ( array $array , mixed $search_value [, bool $strict = FALSE ] )
func fncArrayKeys(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var searchVal **phpv.ZVal
	var strict *phpv.ZBool
	_, err := core.Expand(ctx, args, &array, &searchVal, &strict)
	if err != nil {
		return nil, err
	}

	// TODO: implement strict checking
	_ = strict

	result := phpv.NewZArray()
	iter := array.NewIterator()

	if searchVal != nil {
		for ; iter.Valid(ctx); iter.Next(ctx) {
			key, err := iter.Key(ctx)
			if err != nil {
				return nil, err
			}
			val, err := iter.Current(ctx)
			if err != nil {
				return nil, err
			}

			if val.Value() == (*searchVal).Value() {
				result.OffsetSet(ctx, nil, key)
			}
		}
	} else {
		for key := range array.Iterate(ctx) {
			result.OffsetSet(ctx, nil, key)
		}
	}

	return result.ZVal(), nil
}

// > func array array_flip ( array $array )
func fncArrayFlip(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, err
	}

	result := phpv.NewZArray()

	it := array.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)

		switch v.GetType() {
		case phpv.ZtInt, phpv.ZtString:
		default:
			if err = ctx.Warn("Can only flip string and integer values, entry skipped"); err != nil {
				return nil, err
			}
			continue
		}

		result.OffsetSet(ctx, v, k)
	}

	return result.ZVal(), nil
}

func arrayFilterDefaultCallback(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x *phpv.ZVal
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return x.AsBool(ctx).ZVal(), nil
}

// > func array array_filter ( array $array [, callable $callback [, int $flag = 0 ]] )
func fncArrayFilter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, err
	}

	// In PHP, passing null as callback is equivalent to not passing a callback
	// (filter by truthiness). Handle this before trying to parse as Callable.
	var callback phpv.Callable
	if len(args) >= 2 && args[1] != nil && args[1].GetType() != phpv.ZtNull {
		cb, err := core.SpawnCallableParam(ctx, args[1], 2)
		if err != nil {
			return nil, err
		}
		callback = cb
	} else {
		callback = &phpctx.ExtFunction{
			Func: arrayFilterDefaultCallback,
		}
	}

	var flag phpv.ZInt = 0
	if len(args) >= 3 {
		flagVal, err := args[2].As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		flag = flagVal.Value().(phpv.ZInt)
	}

	result := phpv.NewZArray()

	switch flag {
	case ARRAY_FILTER_USE_BOTH:
		callbackArgs := make([]*phpv.ZVal, 2)
		for k, v := range array.Iterate(ctx) {
			callbackArgs[0] = v
			callbackArgs[1] = k
			ok, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
			if err != nil {
				return nil, err
			}

			if ok.AsBool(ctx) {
				result.OffsetSet(ctx, k, v)
			}
		}
	case ARRAY_FILTER_USE_KEY:
		callbackArgs := make([]*phpv.ZVal, 1)
		for k, v := range array.Iterate(ctx) {
			callbackArgs[0] = k
			ok, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
			if err != nil {
				return nil, err
			}

			if ok.AsBool(ctx) {
				result.OffsetSet(ctx, k, v)
			}
		}
	default:
		callbackArgs := make([]*phpv.ZVal, 1)
		for k, v := range array.Iterate(ctx) {
			callbackArgs[0] = v
			ok, err := ctx.CallZValInternal(ctx, callback, []*phpv.ZVal{v})
			if err != nil {
				return nil, err
			}

			if ok.AsBool(ctx) {
				result.OffsetSet(ctx, k, v)
			}
		}

	}

	return result.ZVal(), nil
}

// > func bool array_walk ( array &$array , callable $callback [, mixed $userdata = NULL ] )
func fncArrayWalk(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			fmt.Sprintf("array_walk() expects at most 3 arguments, %d given", len(args)))
	}
	var array core.Ref[*phpv.ZArray]
	var callback phpv.Callable
	var userdata **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &userdata)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	if userdata != nil {
		callbackArgs = append(callbackArgs, *userdata)
	}

	// array_walk passes elements by reference to the callback,
	// so use CurrentMakeRef to create references to hash table entries.
	arr := array.Get()
	it := arr.NewIterator()
	for it.Valid(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.(interface {
			CurrentMakeRef(phpv.Context) (*phpv.ZVal, error)
		}).CurrentMakeRef(ctx)
		callbackArgs[0] = v
		callbackArgs[1] = k
		_, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
		if err != nil {
			return nil, err
		}
		it.Next(ctx)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool array_walk_recursive ( array &$array , callable $callback [, mixed $userdata = NULL ] )
func fncArrayWalkRecursive(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 3 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			fmt.Sprintf("array_walk_recursive() expects at most 3 arguments, %d given", len(args)))
	}
	var array core.Ref[*phpv.ZArray]
	var callback phpv.Callable
	var userdata **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &userdata)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	if userdata != nil {
		callbackArgs = append(callbackArgs, *userdata)
	}

	var loopErr error
	var loop func(*phpv.ZArray, int)
	loop = func(array *phpv.ZArray, depth int) {
		if depth > 256 || loopErr != nil {
			return
		}
		for k, v := range array.Iterate(ctx) {
			if loopErr != nil {
				return
			}
			if v.GetType() == phpv.ZtArray {
				loop(v.AsArray(ctx), depth+1)
				continue
			}

			callbackArgs[0] = v
			callbackArgs[1] = k
			_, loopErr = ctx.CallZValInternal(ctx, callback, callbackArgs)
		}
	}

	loop(array.Get(), 0)

	if loopErr != nil {
		return nil, loopErr
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func array array_map ( callable $callback , array $array1 [, array $... ] )
func fncArrayMap(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("array_map() expects at least 2 arguments, %d given", len(args))
	}

	// Check if callback is null (special zip mode)
	callbackIsNull := args[0].GetType() == phpv.ZtNull

	var callback phpv.Callable
	if !callbackIsNull {
		var err error
		// Use parent context for callable resolution so that visibility checks
		// use the calling scope's class (not array_map's global scope).
		callerCtx := ctx.Parent(1)
		if callerCtx == nil {
			callerCtx = ctx
		}
		callback, err = core.SpawnCallableParam(callerCtx, args[0], 1)
		if err != nil {
			// If it's a "Cannot call X() dynamically" error, pass through as-is
			if throwErr, ok := err.(*phperr.PhpThrow); ok {
				msg := throwErr.Obj.HashTable().GetString("message").String()
				if strings.HasPrefix(msg, "Cannot call ") && strings.HasSuffix(msg, " dynamically") {
					return nil, err
				}
			}
			// Convert to TypeError with proper array_map() prefix
			cbStr := ""
			if args[0].GetType() == phpv.ZtString {
				cbStr = args[0].String()
			}
			if cbStr != "" {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("array_map(): Argument #1 ($callback) must be a valid callback or null, function \"%s\" not found or invalid function name", cbStr))
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("array_map(): Argument #1 ($callback) must be a valid callback or null, no array, string or object given"))
		}
	}

	// Collect all arrays
	var arrays []*phpv.ZArray
	maxLen := 0
	for i := 1; i < len(args); i++ {
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		arr := b.Value().(*phpv.ZArray)
		arrays = append(arrays, arr)
		if l := int(arr.Count(ctx)); l > maxLen {
			maxLen = l
		}
	}

	result := phpv.NewZArray()

	if callbackIsNull {
		if len(arrays) == 1 {
			// Single array with null callback: return a copy of the array
			for k, v := range arrays[0].Iterate(ctx) {
				result.OffsetSet(ctx, k, v.Dup())
			}
		} else {
			// Multiple arrays: zip into arrays of corresponding elements
			for i := 0; i < maxLen; i++ {
				sub := phpv.NewZArray()
				for _, arr := range arrays {
					val, err := arr.OffsetGet(ctx, phpv.ZInt(i))
					if err != nil {
						sub.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
					} else {
						sub.OffsetSet(ctx, nil, val.Dup())
					}
				}
				result.OffsetSet(ctx, nil, sub.ZVal())
			}
		}
	} else if len(arrays) == 1 {
		// Single array with callback: iterate by actual keys (preserves keys)
		for k, v := range arrays[0].Iterate(ctx) {
			callArgs := []*phpv.ZVal{v}
			val, err := ctx.CallZValInternal(ctx, callback, callArgs)
			if err != nil {
				return nil, err
			}
			err = result.OffsetSet(ctx, k, val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Multiple arrays: iterate by sequential integer index
		for i := 0; i < maxLen; i++ {
			var callArgs []*phpv.ZVal
			for _, arr := range arrays {
				val, err := arr.OffsetGet(ctx, phpv.ZInt(i))
				if err != nil {
					callArgs = append(callArgs, phpv.ZNULL.ZVal())
				} else {
					callArgs = append(callArgs, val)
				}
			}
			val, err := ctx.CallZValInternal(ctx, callback, callArgs)
			if err != nil {
				return nil, err
			}
			err = result.OffsetSet(ctx, nil, val)
			if err != nil {
				return nil, err
			}
		}
	}

	return result.ZVal(), nil
}

// > func array range (  callable $callback , array $array1 [, array $... ] )
// rangeMaxSize is the maximum number of elements range() will produce.
// Matches PHP's HT_MAX_SIZE on 64-bit platforms.
const rangeMaxSize = 1 << 28

func fncRange(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var start, end *phpv.ZVal
	var stepArgVal core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &start, &end, &stepArgVal)
	if err != nil {
		return nil, err
	}

	// Check if both start and end are single-byte strings BEFORE coercion.
	// PHP treats range("1", "9") as a character range producing strings,
	// not a numeric range producing integers.
	isSingleByteRange := rangeIsSingleByteString(start, end)

	// Convert numeric strings to their numeric equivalents.
	// PHP's range() treats "2003" as int 2003 and "1.5" as float 1.5.
	// But NOT single-byte digit strings when both args are single-byte.
	if !isSingleByteRange {
		start = rangeCoerceNumericString(start)
		end = rangeCoerceNumericString(end)
	}

	// Determine if we should use the float path:
	// if any of start, end, or step is a float, use float arithmetic.
	// Exception: if start and end are ints and step is a whole number float,
	// PHP 8.4+ treats it as an integer step and produces integer results.
	useFloat := start.GetType() == phpv.ZtFloat || end.GetType() == phpv.ZtFloat
	if stepArgVal.HasArg() {
		coerced := rangeCoerceNumericString(stepArgVal.Get())
		if coerced.GetType() == phpv.ZtFloat {
			f := float64(coerced.AsFloat(ctx))
			if start.GetType() != phpv.ZtFloat && end.GetType() != phpv.ZtFloat &&
				f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
				// Whole number float step with int endpoints -> use int path
			} else {
				useFloat = true
			}
		}
	}

	// Check for INF/NaN in float arguments
	if start.GetType() == phpv.ZtFloat {
		f := float64(start.AsFloat(ctx))
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("range(): Argument #1 ($start) must be a finite number, %s provided", rangeFormatFloat(f)))
		}
	}
	if end.GetType() == phpv.ZtFloat {
		f := float64(end.AsFloat(ctx))
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("range(): Argument #2 ($end) must be a finite number, %s provided", rangeFormatFloat(f)))
		}
	}

	result := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())

	if start.GetType() == phpv.ZtString && end.GetType() == phpv.ZtString && !useFloat {
		step := 1
		if stepArgVal.HasArg() {
			step = int(stepArgVal.Get().AsInt(ctx))
		}
		if step < 0 {
			step = -step
		}
		if step == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"range(): Argument #3 ($step) must not be 0")
		}

		s1 := []byte(start.AsString(ctx))
		s2 := []byte(end.AsString(ctx))

		// use only first character of the string

		if len(s1) == 0 || len(s2) == 0 {
			if err := result.OffsetSet(ctx, nil, phpv.ZInt(0).ZVal()); err != nil {
				return nil, err
			}
		} else if s1[0] < s2[0] {
			for i := s1[0]; i <= s2[0]; i += byte(step) {
				c := string(rune(i))
				if err := result.OffsetSet(ctx, nil, phpv.ZStr(c)); err != nil {
					return nil, err
				}
			}
		} else {
			for i := s1[0]; i >= s2[0]; i -= byte(step) {
				c := string(rune(i))
				if err := result.OffsetSet(ctx, nil, phpv.ZStr(c)); err != nil {
					return nil, err
				}
			}
		}
	} else if useFloat {
		// Float range path
		f1 := float64(start.AsFloat(ctx))
		f2 := float64(end.AsFloat(ctx))
		fstep := 1.0
		if stepArgVal.HasArg() {
			fstep = float64(stepArgVal.Get().AsFloat(ctx))
		}
		if fstep < 0 {
			fstep = -fstep
		}
		if fstep == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"range(): Argument #3 ($step) must not be 0")
		}

		// Calculate element count before allocating
		var diff float64
		if f1 <= f2 {
			diff = f2 - f1
		} else {
			diff = f1 - f2
		}
		numElementsF := math.Floor(diff/fstep) + 1
		if numElementsF > float64(rangeMaxSize) || math.IsInf(numElementsF, 0) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("The supplied range exceeds the maximum array size by %.1f elements: start=%.1f, end=%.1f, step=%.1f. Max size: %d",
					numElementsF-float64(rangeMaxSize), f1, f2, fstep, rangeMaxSize))
		}

		numElements := int(numElementsF)
		if f1 <= f2 {
			for j := 0; j < numElements; j++ {
				if err := result.OffsetSet(ctx, nil, phpv.ZFloat(f1+float64(j)*fstep).ZVal()); err != nil {
					return nil, err
				}
			}
		} else {
			for j := 0; j < numElements; j++ {
				if err := result.OffsetSet(ctx, nil, phpv.ZFloat(f1-float64(j)*fstep).ZVal()); err != nil {
					return nil, err
				}
			}
		}
	} else {
		step := 1
		if stepArgVal.HasArg() {
			step = int(stepArgVal.Get().AsInt(ctx))
		}

		// Check for INT_MIN step which cannot be negated without overflow
		if step == math.MinInt {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("range(): Argument #3 ($step) must be greater than %d", math.MinInt))
		}

		if step < 0 {
			step = -step
		}

		n1 := int(start.AsInt(ctx))
		n2 := int(end.AsInt(ctx))

		if step == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"range(): Argument #3 ($step) must not be 0")
		}

		// Calculate the number of elements using uint64 to avoid overflow
		// when n1 and n2 are near opposite extremes of the int range.
		// We cast to uint64 before subtraction so the unsigned wraparound
		// gives the correct positive magnitude.
		var numElements uint64
		if n1 < n2 {
			diff := (uint64(n2) - uint64(n1))
			quotient := diff / uint64(step)
			// Guard against +1 overflowing uint64 (e.g., range(MIN, MAX, 1))
			if quotient == math.MaxUint64 {
				numElements = math.MaxUint64
			} else {
				numElements = quotient + 1
			}
		} else if n1 > n2 {
			diff := (uint64(n1) - uint64(n2))
			quotient := diff / uint64(step)
			if quotient == math.MaxUint64 {
				numElements = math.MaxUint64
			} else {
				numElements = quotient + 1
			}
		} else {
			numElements = 1
		}

		if numElements > uint64(rangeMaxSize) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				fmt.Sprintf("The supplied range exceeds the maximum array size by %d elements: start=%d, end=%d, step=%d. Calculated size: %d. Maximum size: %d.",
					numElements-uint64(rangeMaxSize), n1, n2, step, numElements, rangeMaxSize))
		}

		// Use counted loop to avoid overflow when i +/- step wraps around
		// at the boundaries of the int range.
		if n1 <= n2 {
			for j := uint64(0); j < numElements; j++ {
				if err := result.OffsetSet(ctx, nil, phpv.ZInt(n1+int(j)*step).ZVal()); err != nil {
					return nil, err
				}
			}
		} else {
			for j := uint64(0); j < numElements; j++ {
				if err := result.OffsetSet(ctx, nil, phpv.ZInt(n1-int(j)*step).ZVal()); err != nil {
					return nil, err
				}
			}
		}
	}

	return result.ZVal(), nil
}

func rangeFormatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	return "NAN"
}

// rangeCoerceNumericString converts a ZVal that is a numeric string into
// its numeric equivalent (int or float). Non-string or non-numeric values
// are returned as-is. This mirrors PHP's range() behavior where "2003"
// is treated as int 2003 and "1.5" as float 1.5.
func rangeCoerceNumericString(v *phpv.ZVal) *phpv.ZVal {
	if v.GetType() != phpv.ZtString {
		return v
	}
	s := phpv.ZString(v.String())
	numVal, err := s.AsNumeric()
	if err != nil {
		return v
	}
	return numVal.ZVal()
}

// rangeIsSingleByteString checks if BOTH start and end are single-byte strings
// (including digit characters). In that case, range() produces character output.
func rangeIsSingleByteString(start, end *phpv.ZVal) bool {
	if start.GetType() != phpv.ZtString || end.GetType() != phpv.ZtString {
		return false
	}
	return len(start.String()) == 1 && len(end.String()) == 1
}

// > func mixed array_shift ( array &$array )
func fncArrayShift(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0].GetType() != phpv.ZtArray {
		return phpv.ZNULL.ZVal(), ctx.Warn("expects parameter 1 to be array, %s given", args[0].GetType())
	}

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	arr := array.Get()
	if arr.Count(ctx) == 0 {
		return phpv.ZNULL.ZVal(), nil
	}

	// Remove the first element in-place and re-index integer keys.
	// This preserves iterator connections for foreach loops.
	val := arr.HashTable().Shift()
	if val == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	// Reset the internal array pointer after shift, matching PHP behavior
	arr.MainIterator().Reset(ctx)

	return val.ZVal(), nil
}

// > func int array_unshift ( array &$array [, mixed $... ] )
func fncArrayUnshift(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	// Prepend values in-place to preserve iterator connections for foreach loops
	values := make([]*phpv.ZVal, 0, len(args)-1)
	for i := 1; i < len(args); i++ {
		values = append(values, args[i])
	}
	array.Get().HashTable().Unshift(values)

	return phpv.ZInt(array.Get().Count(ctx)).ZVal(), nil
}

// > func int array_push ( array &$array [, mixed $... ] )
func fncArrayPush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for i := 1; i < len(args); i++ {
		err = array.Get().OffsetSet(ctx, nil, args[i])
		if err != nil {
			if err == phpv.ErrNextElementOccupied {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return nil, err
		}
	}

	return phpv.ZInt(array.Get().Count(ctx)).ZVal(), nil
}

// > func mixed array_pop ( array &$array )
func fncArrayPop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0].GetType() != phpv.ZtArray {
		return phpv.ZNULL.ZVal(), ctx.Warn("expects parameter 1 to be array, %s given", args[0].GetType())
	}

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	arr := array.Get()
	if arr.Count(ctx) == 0 {
		return phpv.ZNULL.ZVal(), nil
	}

	var key *phpv.ZVal
	for key = range arr.Iterate(ctx) {
		// iterate until last key
	}

	val, err := arr.OffsetGet(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	err = arr.OffsetUnset(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	// Reset the next integer key counter after pop, matching PHP behavior
	arr.HashTable().RecalcNextIntKey()

	// Reset the internal array pointer after pop, matching PHP behavior
	arr.MainIterator().Reset(ctx)

	return val.ZVal(), nil
}

// > func array array_unique ( array $array [, int $sort_flags = SORT_STRING ] )
func fncArrayUnique(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var sortFlagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &sortFlagsArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	sortFlags := SORT_STRING
	if sortFlagsArg != nil {
		sortFlags = *sortFlagsArg
	}

	result := phpv.NewZArray()

	switch sortFlags {
	case SORT_REGULAR:
		// Collect seen values and compare each new value against all seen ones.
		// This is needed because objects (e.g. enums) cannot be used as array keys.
		var seen []*phpv.ZVal
		for k, v := range array.Iterate(ctx) {
			found := false
			for _, sv := range seen {
				eq, err := phpv.Equals(ctx, v, sv)
				if err != nil {
					return nil, err
				}
				if eq {
					found = true
					break
				}
			}
			if !found {
				seen = append(seen, v)
				result.OffsetSet(ctx, k, v)
			}
		}

	case SORT_NUMERIC:
		added := map[phpv.ZInt]struct{}{}
		for k, v := range array.Iterate(ctx) {
			n := v.AsInt(ctx)
			if _, ok := added[n]; ok {
				continue
			}
			added[n] = struct{}{}
			result.OffsetSet(ctx, k, v)
		}

	case SORT_STRING, SORT_LOCALE_STRING:
		fallthrough
	default:
		added := map[phpv.ZString]struct{}{}
		for k, v := range array.Iterate(ctx) {
			s := v.AsString(ctx)
			if _, ok := added[s]; ok {
				continue
			}
			added[s] = struct{}{}
			result.OffsetSet(ctx, k, v)
		}
	}

	return result.ZVal(), nil
}

// > func array array_slice ( array $array , int $offset [, int $length = NULL [, bool $preserve_keys = FALSE ]] )
func fncArraySlice(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate $length parameter type before Expand (PHP rejects arrays and objects)
	// Also track whether length was explicitly null.
	lengthIsNull := false
	if len(args) >= 3 && args[2] != nil {
		if args[2].IsNull() {
			lengthIsNull = true
		} else {
			t := args[2].GetType()
			switch t {
			case phpv.ZtArray, phpv.ZtObject, phpv.ZtString:
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("array_slice(): Argument #3 ($length) must be of type ?int, %s given",
						phpv.ZValTypeName(args[2])))
			case phpv.ZtFloat:
				// In strict_types mode, float is also rejected for int parameters
				strictTypes := ctx.GetConfig("strict_types", phpv.ZInt(0).ZVal())
				if strictTypes != nil && strictTypes.AsInt(ctx) == 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"array_slice(): Argument #3 ($length) must be of type ?int, float given")
				}
			}
		}
	}
	var array *phpv.ZArray
	var offset phpv.ZInt
	var lengthArg *phpv.ZInt
	var preserveKeysArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &array, &offset, &lengthArg, &preserveKeysArg)
	if err != nil {
		return nil, ctx.Error(err)
	}
	// If length was explicitly null, treat as if it wasn't provided (take all)
	if lengthIsNull {
		lengthArg = nil
	}

	arrayCount := array.Count(ctx)
	length := phpv.ZInt(arrayCount)
	preserveKeys := false

	if offset < 0 {
		offset = arrayCount + offset
	}
	if preserveKeysArg != nil {
		preserveKeys = bool(*preserveKeysArg)
	}
	if lengthArg != nil {
		length = *lengthArg
		if length < 0 {
			length = min(arrayCount+length, arrayCount)
		} else {
			length = min(offset+length, arrayCount)
		}
	}

	offset = max(offset, 0)
	end := min(length, arrayCount)

	result := phpv.NewZArray()

	i := phpv.ZInt(-1)
	j := phpv.ZInt(0)
	for key, val := range array.Iterate(ctx) {
		i++
		if i < offset {
			continue
		}
		if i >= end {
			break
		}

		switch key.GetType() {
		case phpv.ZtInt:
			if preserveKeys {
				result.OffsetSet(ctx, key, val)
			} else {
				result.OffsetSet(ctx, phpv.ZInt(j), val)
				j++
			}
		case phpv.ZtString:
			result.OffsetSet(ctx, key, val)
		}

	}

	return result.ZVal(), nil
}

// > func mixed array_search ( mixed $needle , array $haystack [, bool $strict = FALSE ] )
func fncArraySearch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var needle *phpv.ZVal
	var haystack *phpv.ZArray
	var strict *phpv.ZBool

	_, err := core.Expand(ctx, args, &needle, &haystack, &strict)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	if strict != nil && *strict {
		for k, v := range haystack.Iterate(ctx) {
			eq, err := phpv.StrictEquals(ctx, needle, v)
			if err != nil {
				return nil, err
			}
			if eq {
				return k, nil
			}
		}
	} else {
		for k, v := range haystack.Iterate(ctx) {
			eq, err := phpv.Equals(ctx, needle, v)
			if err != nil {
				return nil, err
			}
			if eq {
				return k, nil
			}
		}
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func mixed key ( array $array )
func fncArrayKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling key() on an object is deprecated")
	}
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	key, err := array.MainIterator().Key(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if key == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return key, nil
}

// > func mixed current ( array $array )
// > alias pos
func fncArrayCurrent(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling current() on an object is deprecated")
	}
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	current, err := array.MainIterator().Current(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	// Return the value, not the reference
	return current.Nude(), nil
}

// > func mixed next ( array &$array )
func fncArrayNext(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// PHP 8.5: calling next() on an object is deprecated
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling next() on an object is deprecated")
	}

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	current, err := array.Get().MainIterator().Next(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}

	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return current, nil
}

// > func mixed prev ( array &$array )
func fncArrayPrev(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling prev() on an object is deprecated")
	}
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	current, err := array.Get().MainIterator().Prev(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return current, nil
}

// > func mixed reset ( array &$array )
func fncArrayReset(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling reset() on an object is deprecated")
	}
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if array.Get().Count(ctx) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	current, err := array.Get().MainIterator().Reset(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return current, nil
}

// > func mixed end ( array &$array )
func fncArrayEnd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtObject {
		ctx.Deprecated("Calling end() on an object is deprecated")
	}
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if array.Get().Count(ctx) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	current, err := array.Get().MainIterator().End(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return current, nil
}

// > func mixed each ( array &$array )
func fncArrayEach(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if err := ctx.WarnDeprecated(); err != nil {
		return nil, err
	}

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if array.Get().Count(ctx) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	current, err := array.Get().MainIterator().Current(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	if current == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	key, err := array.Get().MainIterator().Key(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), current)
	result.OffsetSet(ctx, phpv.ZStr("value"), current)
	result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), key)
	result.OffsetSet(ctx, phpv.ZStr("key"), key)

	_, err = array.Get().MainIterator().Next(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}

	return result.ZVal(), nil
}

// > func array array_reverse ( array $array1 [, bool $preserve_keys = FALSE ] )
func fncArrayReverse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var preserveKeysArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &array, &preserveKeysArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	preserveKeys := false
	if preserveKeysArg != nil {
		preserveKeys = bool(*preserveKeysArg)
	}

	result := phpv.NewZArray()
	it := array.NewIterator()
	it.End(ctx)
	i := 0
	for it.Valid(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)

		if k.GetType() == phpv.ZtInt && !preserveKeys {
			k = phpv.ZInt(i).ZVal()
			result.OffsetSet(ctx, k, v)
			i++
		} else {
			result.OffsetSet(ctx, k, v)
		}

		it.Prev(ctx)
	}

	return result.ZVal(), nil
}

// > func array array_change_key_case ( array $array1 [, int $case = CASE_LOWER ] )
func fncArrayChangeKeyCase(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var keyCaseArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &keyCaseArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	keyCase := core.Deref(keyCaseArg, CASE_LOWER)

	result := phpv.NewZArray()
	for k, v := range array.Iterate(ctx) {
		if k.GetType() == phpv.ZtString {
			s := k.AsString(ctx)
			changeCase := core.IfElse(keyCase == CASE_LOWER, s.ToLower, s.ToUpper)
			k = changeCase().ZVal()
		}
		result.OffsetSet(ctx, k, v)
	}

	return result.ZVal(), nil
}

// > func array array_chunk ( array $array , int $size [, bool $preserve_keys = FALSE ] )
func fncArrayChunk(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var size phpv.ZInt
	var preserveKeysArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &array, &size, &preserveKeysArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if size <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_chunk(): Argument #2 ($length) must be greater than 0")
	}

	preserveKeys := core.Deref(preserveKeysArg, false)

	result := phpv.NewZArray()
	current := phpv.NewZArray()
	for k, v := range array.Iterate(ctx) {
		if current.Count(ctx) >= size {
			result.OffsetSet(ctx, nil, current.ZVal())
			current = phpv.NewZArray()
		}

		if preserveKeys {
			current.OffsetSet(ctx, k, v)
		} else {
			current.OffsetSet(ctx, nil, v)
		}
	}
	if current.Count(ctx) > 0 {
		result.OffsetSet(ctx, nil, current.ZVal())
	}

	return result.ZVal(), nil
}

func getArrayKeyValue(ctx phpv.Context, s *phpv.ZVal) (*phpv.ZVal, error) {
	switch s.GetType() {
	case phpv.ZtNull:
		return phpv.ZInt(0).ZVal(), nil
	case phpv.ZtBool:
		if s.Value().(phpv.ZBool) {
			return phpv.ZInt(1).ZVal(), nil
		} else {
			return phpv.ZInt(0).ZVal(), nil
		}
	case phpv.ZtFloat:
		n := int(s.Value().(phpv.ZFloat))
		return phpv.ZInt(n).ZVal(), nil
	case phpv.ZtInt:
		return s.Value().(phpv.ZInt).ZVal(), nil
	case phpv.ZtObject:
		var err error
		s, err = s.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		fallthrough
	case phpv.ZtString:
		str := s.String()
		if phpv.ZString(str).LooksInt() {
			i, err := strconv.ParseInt(str, 10, 64)
			if err == nil {
				// check if converting back results in same value
				s2 := strconv.FormatInt(i, 10)
				if str == s2 {
					// ok, we can use zint
					return phpv.ZInt(i).ZVal(), nil
				}
			}
		}

		return phpv.ZString(str).ZVal(), nil

	default:
		return phpv.ZString("").ZVal(), nil
	}
}

// > func array array_column ( array $input , mixed $column_key [, mixed $index_key = NULL ] )
func fncArrayColumn(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var columnKeyArg *phpv.ZVal
	var indexKeyArg core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &array, &columnKeyArg, &indexKeyArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	// When column_key is null, we return the entire row
	columnKeyIsNull := columnKeyArg == nil || columnKeyArg.IsNull()

	var columnKey *phpv.ZVal
	if !columnKeyIsNull {
		columnKey, err = getArrayKeyValue(ctx, columnKeyArg)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
	}

	var indexKey *phpv.ZVal
	var indexKeyIsNull bool
	if indexKeyArg.HasArg() {
		ik := indexKeyArg.Get()
		indexKeyIsNull = ik == nil || ik.IsNull()
		if !indexKeyIsNull {
			indexKey, err = getArrayKeyValue(ctx, ik)
			if err != nil {
				return nil, ctx.FuncError(err)
			}
		}
	} else {
		indexKeyIsNull = true
	}

	// result is an array of { row[indexKey] : row[columnKey], ... }
	// where row = array[i]
	// if column_key is null, return the entire row
	// if row[indexKey] doesn't exist or non-numeric, use maxIndex+1 as key

	result := phpv.NewZArray()
	for _, item := range array.Iterate(ctx) {
		// array_column also works on objects (accessing public properties)
		var value *phpv.ZVal
		if columnKeyIsNull {
			value = item
		} else {
			if item.GetType() == phpv.ZtArray {
				row := item.AsArray(ctx)
				if exists, _ := row.OffsetExists(ctx, columnKey); !exists {
					continue
				}
				value, _ = row.OffsetGet(ctx, columnKey)
			} else if item.GetType() == phpv.ZtObject {
				// Access object property
				obj, ok := item.Value().(*phpobj.ZObject)
				if !ok {
					continue
				}
				propName := columnKey.AsString(ctx)
				propVal, err := obj.ObjectGet(ctx, propName)
				if err != nil || propVal == nil {
					continue
				}
				value = propVal
			} else {
				continue
			}
		}

		var key *phpv.ZVal
		if !indexKeyIsNull && indexKey != nil {
			if item.GetType() == phpv.ZtArray {
				row := item.AsArray(ctx)
				if exists, _ := row.OffsetExists(ctx, indexKey); exists {
					k, _ := row.OffsetGet(ctx, indexKey)
					if k.GetType() == phpv.ZtInt {
						index := k.AsInt(ctx)
						key = index.ZVal()
					} else {
						key = k
					}
				}
			} else if item.GetType() == phpv.ZtObject {
				obj, ok := item.Value().(*phpobj.ZObject)
				if ok {
					propName := indexKey.AsString(ctx)
					propVal, err := obj.ObjectGet(ctx, propName)
					if err == nil && propVal != nil {
						if propVal.GetType() == phpv.ZtInt {
							key = propVal
						} else {
							key = propVal
						}
					}
				}
			}
		}
		result.OffsetSet(ctx, key, value)
	}

	return result.ZVal(), nil
}

// > func array array_count_values ( $array )
func fncArrayCountValues(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	for _, v := range array.Iterate(ctx) {
		switch v.GetType() {
		case phpv.ZtInt:
		case phpv.ZtString:
		default:
			if err = ctx.Warn("Can only count string and integer values, entry skipped"); err != nil {
				return nil, err
			}
			continue
		}

		countVal, exists, _ := result.OffsetCheck(ctx, v)
		if exists {
			n := countVal.AsInt(ctx) + 1
			result.OffsetSet(ctx, v, n.ZVal())
		} else {
			result.OffsetSet(ctx, v, phpv.ZInt(1).ZVal())
		}
	}
	return result.ZVal(), nil
}

// > func array array_fill ( int $start_index , int $num , mixed $value )
func fncArrayFill(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var startIndex, num phpv.ZInt
	var fillValue *phpv.ZVal
	_, err := core.Expand(ctx, args, &startIndex, &num, &fillValue)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if num < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_fill(): Argument #2 ($count) must be greater than or equal to 0")
	}
	if num > 10000000 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_fill(): Argument #2 ($count) is too large")
	}
	result := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
	for i := startIndex; i < startIndex+num; i++ {
		if i%10000 == 0 {
			if err := ctx.Tick(ctx, nil); err != nil {
				return nil, err
			}
		}
		if err := result.OffsetSet(ctx, phpv.ZInt(i), fillValue); err != nil {
			return nil, err
		}
	}
	return result.ZVal(), nil
}

// > func array array_fill_keys ( array $keys , mixed $value )
func fncArrayFillKeys(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var fillValue *phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &fillValue)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
	for _, v := range array.Iterate(ctx) {
		// array_fill_keys converts values to string first for use as keys,
		// so float 1.23 becomes string key "1.23" rather than int key 1.
		keyStr, err := v.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		if err := result.OffsetSet(ctx, keyStr, fillValue); err != nil {
			return nil, err
		}
	}
	return result.ZVal(), nil
}

// > func mixed array_first ( array $array )
func fncArrayFirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for _, v := range array.Iterate(ctx) {
		return v, nil
	}

	return phpv.ZNULL.ZVal(), nil
}

// > func mixed array_last ( array $array )
func fncArrayLast(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	it := array.NewIterator()
	_, err = it.End(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if !it.Valid(ctx) {
		return phpv.ZNULL.ZVal(), nil
	}

	v, err := it.Current(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return v, nil
}

// > func array array_key_first ( array $keys )
func fncArrayKeyFirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for k := range array.Iterate(ctx) {
		return k, nil
	}

	return phpv.ZNULL.ZVal(), nil
}

// > func array array_key_last ( array $keys )
func fncArrayKeyLast(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	it := array.NewIterator()
	_, err = it.End(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	k, err := it.Key(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return k, nil
}

// > func array array_merge_recursive ( array $array1 [, array $... ] )
func fncArrayMergeRecursive(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate all arguments are arrays first
	for i, arg := range args {
		if arg.GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("array_merge_recursive(): Argument #%d must be of type array, %s given", i+1, arg.GetType().TypeName()))
		}
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	arrayRecursiveMerge(ctx, result, array)
	for _, elem := range args[1:] {
		arr, err := elem.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arrayRecursiveMerge(ctx, result, arr.AsArray(ctx))
	}

	return result.ZVal(), nil
}

func arrayRecursiveMerge(ctx phpv.Context, result, array *phpv.ZArray, depth ...int) {
	d := 0
	if len(depth) > 0 {
		d = depth[0]
	}
	if d > 256 {
		return
	}
	for k, v := range array.Iterate(ctx) {
		if k.GetType() == phpv.ZtInt {
			result.OffsetSet(ctx, nil, v)
			continue
		}

		if v.GetType() == phpv.ZtArray {
			var array *phpv.ZArray
			cur, _ := result.OffsetGet(ctx, k)
			if cur.GetType() != phpv.ZtArray {
				array = phpv.NewZArray()
				result.OffsetSet(ctx, k, array.ZVal())
			} else {
				array = cur.AsArray(ctx)
			}

			arrayRecursiveMerge(ctx, array, v.AsArray(ctx), d+1)
			continue
		}

		if ok, _ := result.OffsetExists(ctx, k); ok {
			cur, _ := result.OffsetGet(ctx, k)
			if cur.GetType() != phpv.ZtArray {
				array := phpv.NewZArray()
				result.OffsetUnset(ctx, k)
				array.OffsetSet(ctx, nil, cur)
				array.OffsetSet(ctx, nil, v)
				result.OffsetSet(ctx, k, array.ZVal())
			} else {
				array := cur.AsArray(ctx)
				array.OffsetSet(ctx, nil, v)
			}
			continue
		}

		result.OffsetSet(ctx, k, v)
	}
}

// > func array array_replace_recursive ( array $array1 [, array $... ] )
func fncArrayReplaceRecursive(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	arrayRecursiveReplace(ctx, result, array)
	for _, elem := range args[1:] {
		arr, err := elem.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arrayRecursiveReplace(ctx, result, arr.AsArray(ctx))
	}

	return result.ZVal(), nil
}

func arrayRecursiveReplace(ctx phpv.Context, result, array *phpv.ZArray, depth ...int) {
	d := 0
	if len(depth) > 0 {
		d = depth[0]
	}
	if d > 256 {
		return // prevent infinite recursion on circular references
	}
	for k, v := range array.Iterate(ctx) {
		if v.GetType() == phpv.ZtArray {
			var array *phpv.ZArray
			cur, _ := result.OffsetGet(ctx, k)
			if cur.GetType() != phpv.ZtArray {
				array = phpv.NewZArray()
				result.OffsetSet(ctx, k, array.ZVal())
			} else {
				array = cur.AsArray(ctx)
			}

			arrayRecursiveReplace(ctx, array, v.AsArray(ctx), d+1)
			result.OffsetSet(ctx, k, array.ZVal())
			continue
		}
		result.OffsetSet(ctx, k, v)
	}
}

// > func array array_pad ( array $array , int $size , mixed $value )
func fncArrayPad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var size phpv.ZInt
	var padValue *phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &size, &padValue)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	// PHP limits array_pad to a reasonable size
	const maxPadSize = 1048576 // 1M elements max
	absSize := int64(size)
	if absSize < 0 {
		if absSize == math.MinInt64 {
			absSize = math.MaxInt64
		} else {
			absSize = -absSize
		}
	}
	if absSize > maxPadSize {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_pad(): Argument #2 ($length) must not exceed the maximum allowed array size")
	}

	var result *phpv.ZArray
	if size < 0 {
		padCount := int(absSize) - int(array.Count(ctx))
		if padCount < 0 {
			padCount = 0
		}
		result = phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())
		for i := 0; i < padCount; i++ {
			if err := result.OffsetSet(ctx, nil, padValue); err != nil {
				return nil, err
			}
		}
		for k, v := range array.Iterate(ctx) {
			if k.GetType() == phpv.ZtInt {
				if err := result.OffsetSet(ctx, nil, v); err != nil {
					return nil, err
				}
			} else {
				if err := result.OffsetSet(ctx, k, v); err != nil {
					return nil, err
				}
			}
		}
	} else {
		padCount := int(size) - int(array.Count(ctx))
		if padCount < 0 {
			padCount = 0
		}
		result = array.Dup()
		for i := 0; i < padCount; i++ {
			if err := result.OffsetSet(ctx, nil, padValue); err != nil {
				return nil, err
			}
		}
	}

	return result.ZVal(), nil
}

// > func number array_product ( array $array )
func fncArrayProduct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	// PHP returns int when all values are integers, float otherwise
	floatResult := false
	var intProduct phpv.ZInt = 1
	var floatProduct phpv.ZFloat = 1
	for _, v := range array.Iterate(ctx) {
		switch v.GetType() {
		case phpv.ZtArray, phpv.ZtObject:
			if err := ctx.Warn("Multiplication is not supported on type %s", v.GetType().TypeName()); err != nil {
				return nil, err
			}
			continue
		case phpv.ZtString:
			// Check if string is numeric; if not, warn but still use its numeric value (0)
			s := v.AsString(ctx)
			isNumStr := s.IsNumeric()
			if !isNumStr {
				if err := ctx.Warn("Multiplication is not supported on type string"); err != nil {
					return nil, err
				}
			}
			ss := string(s)
			// Only check for float indicators in actually numeric strings
			if isNumStr && strings.ContainsAny(ss, ".eE") {
				floatResult = true
			}
			if !floatResult {
				vi := v.AsInt(ctx)
				newProduct := intProduct * vi
				if vi != 0 && intProduct != 0 && newProduct/vi != intProduct {
					floatResult = true
				} else {
					intProduct = newProduct
				}
			}
			floatProduct *= v.AsFloat(ctx)
		case phpv.ZtFloat:
			floatResult = true
			floatProduct *= v.AsFloat(ctx)
		case phpv.ZtBool:
			boolVal := v.AsInt(ctx)
			intProduct *= boolVal
			floatProduct *= phpv.ZFloat(boolVal)
		case phpv.ZtResource:
			if err := ctx.Warn("Multiplication is not supported on type %s", v.GetType().TypeName()); err != nil {
				return nil, err
			}
			// Warn but still use the resource's numeric value (resource ID)
			vi := v.AsInt(ctx)
			intProduct *= vi
			floatProduct *= phpv.ZFloat(vi)
		default:
			if !floatResult {
				newProduct := intProduct * v.AsInt(ctx)
				// Detect overflow: if v != 0 and result/v != intProduct, overflow occurred
				vi := v.AsInt(ctx)
				if vi != 0 && intProduct != 0 && newProduct/vi != intProduct {
					floatResult = true
				} else {
					intProduct = newProduct
				}
			}
			floatProduct *= v.AsFloat(ctx)
		}
	}

	if floatResult {
		return floatProduct.ZVal(), nil
	}
	return intProduct.ZVal(), nil
}

// > func number array_sum ( array $array )
func fncArraySum(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	floatResult := false
	var sum phpv.ZFloat = 0
	for _, v := range array.Iterate(ctx) {
		if v.GetType() == phpv.ZtArray {
			continue
		}

		floatResult = floatResult || v.GetType() == phpv.ZtFloat
		sum += v.AsFloat(ctx)
	}

	if !floatResult {
		// Check if sum fits in int64 range
		if sum >= -9223372036854775808 && sum <= 9223372036854775807 {
			return phpv.ZInt(sum).ZVal(), nil
		}
		// Overflow - return as float
	}

	return sum.ZVal(), nil
}

// > func mixed array_rand ( array $array [, int $num = 1 ] )
func fncArrayRand(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var numArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &numArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if array.Count(ctx) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_rand(): Argument #1 ($array) must not be empty")
	}

	// TODO: use Mersenne Twister RNG for maximum compatibility

	num := core.Deref(numArg, 1)

	if num < 1 || num > phpv.ZInt(array.Count(ctx)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"array_rand(): Argument #2 ($num) must be between 1 and the number of elements in argument #1 ($array)")
	}

	if num == 1 {
		i := rand.IntN(int(array.Count(ctx)))
		return array.OffsetKeyAt(ctx, i)
	}

	result := phpv.NewZArray()
	indices := rand.Perm(int(array.Count(ctx)))[:int(num)]

	i := 0
	for k := range array.Iterate(ctx) {
		if slices.Contains(indices, i) {
			result.OffsetSet(ctx, nil, k)
		}
		i++
	}

	return result.ZVal(), nil
}

// > func mixed array_reduce ( array $array , callable $callback [, mixed $initial = NULL ] )
func fncArrayReduce(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	var initialArg **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &initialArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	accumulator := core.Deref(initialArg, phpv.ZNULL.ZVal())

	cbArgs := make([]*phpv.ZVal, 2)
	for _, v := range array.Iterate(ctx) {
		cbArgs[0] = accumulator
		cbArgs[1] = v
		accumulator, err = ctx.CallZValInternal(ctx, callback, cbArgs)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
	}

	return accumulator, nil
}

// > func mixed extract ( array &$array [, int $flags = EXTR_OVERWRITE [, string $prefix = NULL ]] )
func fncArrayExtract(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var flagsArg *phpv.ZInt
	var prefixArgs *phpv.ZString
	_, err := core.Expand(ctx, args, &array, &flagsArg, &prefixArgs)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	parentCtx := ctx.Parent(1)
	flags := core.Deref(flagsArg, EXTR_OVERWRITE)
	prefix := string(core.Deref(prefixArgs, ""))

	// Strip EXTR_REFS flag
	baseFlags := flags & ^EXTR_REFS

	// Validate flags
	switch baseFlags {
	case EXTR_OVERWRITE, EXTR_SKIP, EXTR_PREFIX_SAME, EXTR_PREFIX_ALL,
		EXTR_PREFIX_INVALID, EXTR_PREFIX_IF_EXISTS, EXTR_IF_EXISTS:
		// valid
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"extract(): Argument #2 ($flags) must be a valid extract type")
	}

	// Prefix is required for certain modes - but only if it was not provided at all
	switch baseFlags {
	case EXTR_PREFIX_SAME, EXTR_PREFIX_ALL, EXTR_PREFIX_INVALID, EXTR_PREFIX_IF_EXISTS:
		if prefixArgs == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"extract(): Argument #3 ($prefix) is required when using this extract type")
		}
	}

	count := phpv.ZInt(0)

	for k, v := range array.Get().Iterate(ctx) {
		varName := string(k.AsString(ctx))

		invalidVarName := k.GetType() == phpv.ZtInt
		if !invalidVarName && !extractIsValidVarName(varName) {
			invalidVarName = true
		}

		switch baseFlags {
		case EXTR_OVERWRITE, EXTR_SKIP, EXTR_IF_EXISTS:
			if invalidVarName {
				continue
			}
		}

		alreadyDefined, _ := parentCtx.OffsetExists(ctx, phpv.ZString(varName).ZVal())

		var targetName string
		doSet := false

		switch baseFlags {
		case EXTR_OVERWRITE:
			targetName = varName
			doSet = true
		case EXTR_SKIP:
			if !alreadyDefined {
				targetName = varName
				doSet = true
			}
		case EXTR_PREFIX_SAME:
			if alreadyDefined {
				targetName = prefix + "_" + varName
				doSet = true
			} else if !invalidVarName {
				targetName = varName
				doSet = true
			}
		case EXTR_PREFIX_ALL:
			targetName = prefix + "_" + varName
			doSet = true
		case EXTR_PREFIX_INVALID:
			if invalidVarName {
				targetName = prefix + "_" + varName
				doSet = true
			} else {
				targetName = varName
				doSet = true
			}
		case EXTR_IF_EXISTS:
			if alreadyDefined {
				targetName = varName
				doSet = true
			}
		case EXTR_PREFIX_IF_EXISTS:
			if alreadyDefined {
				targetName = prefix + "_" + varName
				doSet = true
			}
		}

		if doSet && extractIsValidVarName(targetName) {
			parentCtx.OffsetSet(parentCtx, phpv.ZString(targetName).ZVal(), v)
			count++
		}
	}

	return count.ZVal(), nil
}

func extractIsValidVarName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80) {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c >= 0x80) {
				return false
			}
		}
	}
	return true
}

const compactMaxDepth = 32

// > func array compact ( mixed $varname1 [, mixed $... ] )
func fncArrayCompact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return nil, ctx.Warn("expects at least 1 parameter, 0 given")
	}
	parentCtx := ctx.Parent(1)
	result := phpv.NewZArray()
	for i, v := range args {
		err := arrayRecursiveCompact(ctx, parentCtx, result, v, 0, i+1)
		if err != nil {
			return nil, err
		}
	}
	return result.ZVal(), nil
}

func arrayRecursiveCompact(funcCtx phpv.Context, ctx phpv.Context, result *phpv.ZArray, varName *phpv.ZVal, depth int, argNum int) error {
	switch varName.GetType() {
	case phpv.ZtString:
		if ok, _ := ctx.OffsetExists(ctx, varName); ok {
			value, err := ctx.OffsetGet(ctx, varName)
			if err != nil {
				return err
			}
			result.OffsetSet(ctx, varName, value)
		} else {
			funcCtx.Notice("compact(): Undefined variable: %s", varName, logopt.NoFuncName(true))
		}
	case phpv.ZtArray:
		if depth >= compactMaxDepth {
			return phpobj.ThrowError(ctx, phpobj.Error, "Recursion detected")
		}
		arr := varName.AsArray(ctx)
		for _, varName := range arr.Iterate(ctx) {
			err := arrayRecursiveCompact(funcCtx, ctx, result, varName, depth+1, argNum)
			if err != nil {
				return err
			}
		}
	default:
		// PHP silently ignores non-string, non-array values
	}

	return nil
}


// > func mixed shuffle ( array &$array )
func fncArrayShuffle(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.FuncError(err)
	}

	var values []*phpv.ZVal
	for _, v := range array.Get().Iterate(ctx) {
		values = append(values, v)
	}
	sort.Slice(values, func(_, _ int) bool {
		return rand.IntN(2) == 1
	})

	array.Get().Clear(ctx)
	for _, v := range values {
		array.Get().OffsetSet(ctx, nil, v)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func array array_splice ( array &$input , int $offset [, int $length = count($input) [, mixed $replacement = array() ]] )
func fncArraySplice(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var offset phpv.ZInt
	var lengthArg core.Optional[phpv.ZInt]
	var replacementArg core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &array, &offset, &lengthArg, &replacementArg)
	if err != nil {
		return nil, ctx.Error(err)
	}

	// Check if length argument was explicitly null (treated same as omitted in PHP)
	lengthIsNull := len(args) >= 3 && args[2] != nil && args[2].GetType() == phpv.ZtNull

	arrayCount := array.Get().Count(ctx)
	length := phpv.ZInt(arrayCount)
	replacement := phpv.NewZArray()

	if offset < 0 {
		offset = arrayCount + offset
	}
	if lengthArg.HasArg() && !lengthIsNull {
		length = lengthArg.Get()
		if length < 0 {
			length = min(arrayCount+length, arrayCount)
		} else {
			length = min(offset+length, arrayCount)
		}
	}

	if replacementArg.HasArg() {
		if replacementArg.Get().GetType() == phpv.ZtArray {
			arr := replacementArg.Get().AsArray(ctx)
			for _, v := range arr.Iterate(ctx) {
				replacement.OffsetSet(ctx, nil, v)
			}
		} else {
			replacement.OffsetSet(ctx, nil, replacementArg.Get())
		}
	}

	offset = max(offset, 0)
	end := min(length, arrayCount)
	result := phpv.NewZArray()

	it := array.Get().NewIterator()
	array.Get().Empty(ctx)

	i := phpv.ZInt(-1)
	j := phpv.ZInt(0)

	for k, v := range it.Iterate(ctx) {
		i++
		if i < offset || (i >= end && offset != end) {
			if k.GetType() == phpv.ZtInt {
				array.Get().OffsetSet(ctx, j, v)
				j++
			} else {
				array.Get().OffsetSet(ctx, k, v)
			}
		} else {
			if i == offset {
				for _, v := range replacement.Iterate(ctx) {
					array.Get().OffsetSet(ctx, j, v)
					j++
				}
			}
			if offset == end {
				array.Get().OffsetSet(ctx, nil, v)
				j++
			} else {
				result.OffsetSet(ctx, nil, v)
			}
		}
	}

	return result.ZVal(), nil
}

// > func bool array_is_list ( array $array )
func fncArrayIsList(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"array_is_list() expects exactly 1 argument, 0 given")
	}
	if args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("array_is_list(): Argument #1 ($array) must be of type array, %s given",
				phpv.ZValTypeNameDetailed(args[0])))
	}

	arr := args[0].AsArray(ctx)
	expectedKey := phpv.ZInt(0)
	it := arr.NewIterator()
	for it.Valid(ctx) {
		k, err := it.Key(ctx)
		if err != nil {
			return nil, err
		}
		if k.GetType() != phpv.ZtInt || k.Value().(phpv.ZInt) != expectedKey {
			return phpv.ZBool(false).ZVal(), nil
		}
		expectedKey++
		it.Next(ctx)
	}
	return phpv.ZBool(true).ZVal(), nil
}

// > func bool array_any ( array $array , callable $callback )
func fncArrayAny(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &array, &callback)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	for k, v := range array.Iterate(ctx) {
		callbackArgs[0] = v
		callbackArgs[1] = k
		result, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
		if err != nil {
			return nil, err
		}
		if result.AsBool(ctx) {
			return phpv.ZBool(true).ZVal(), nil
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

// > func bool array_all ( array $array , callable $callback )
func fncArrayAll(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &array, &callback)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	for k, v := range array.Iterate(ctx) {
		callbackArgs[0] = v
		callbackArgs[1] = k
		result, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
		if err != nil {
			return nil, err
		}
		if !result.AsBool(ctx) {
			return phpv.ZBool(false).ZVal(), nil
		}
	}
	return phpv.ZBool(true).ZVal(), nil
}

// > func mixed array_find ( array $array , callable $callback )
func fncArrayFind(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &array, &callback)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	for k, v := range array.Iterate(ctx) {
		callbackArgs[0] = v
		callbackArgs[1] = k
		result, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
		if err != nil {
			return nil, err
		}
		if result.AsBool(ctx) {
			return v, nil
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// > func mixed array_find_key ( array $array , callable $callback )
func fncArrayFindKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &array, &callback)
	if err != nil {
		return nil, err
	}

	callbackArgs := make([]*phpv.ZVal, 2)
	for k, v := range array.Iterate(ctx) {
		callbackArgs[0] = v
		callbackArgs[1] = k
		result, err := ctx.CallZValInternal(ctx, callback, callbackArgs)
		if err != nil {
			return nil, err
		}
		if result.AsBool(ctx) {
			return k, nil
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

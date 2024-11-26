package standard

import (
	"errors"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	ARRAY_FILTER_USE_KEY  phpv.ZInt = 1
	ARRAY_FILTER_USE_BOTH phpv.ZInt = 2

	SORT_REGULAR       phpv.ZInt = 0
	SORT_NUMERIC       phpv.ZInt = 1
	SORT_STRING        phpv.ZInt = 2
	SORT_LOCALE_STRING phpv.ZInt = 5
)

// > func array array_combine ( array $keys , array $values )
func fncArrayCombine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var keys, values *phpv.ZArray
	_, err := core.Expand(ctx, args, &keys, &values)
	if err != nil {
		return nil, err
	}

	if keys.Count(ctx) != values.Count(ctx) {
		return nil, errors.New("Argument #1 ($keys) and argument #2 ($values) must have the same number of elements")
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
		err = result.OffsetSet(ctx, key, val)
		if err != nil {
			return nil, err
		}

		err = keyIter.Next(ctx)
		if err != nil {
			return nil, err
		}
		err = valIter.Next(ctx)
		if err != nil {
			return nil, err
		}
	}

	return result.ZVal(), nil
}

// > func array array_merge ( array $array1 [, array $... ] )
func fncArrayMerge(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZArray
	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}
	a = a.Dup() // make sure we do a copy of array

	for i := 1; i < len(args); i++ {
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		err = a.MergeTable(b.HashTable())
		if err != nil {
			return nil, err
		}
	}

	return a.ZVal(), nil
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

		// TODO: doesn't work with non-scalar types
		if strict {
			if needle.GetType() != val.GetType() && needle.Value() == val.Value() {
				return phpv.ZBool(true).ZVal(), nil
			}
		} else {
			switch needle.GetType() {
			case phpv.ZtBool, phpv.ZtFloat, phpv.ZtInt, phpv.ZtNull, phpv.ZtString:
				if needle.String() == val.String() {
					return phpv.ZBool(true).ZVal(), nil
				}
			default:
				if needle.Value() == val.Value() {
					return phpv.ZBool(true).ZVal(), nil
				}
			}
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

// > func bool array_key_exists (  mixed $key , array $array )
func fncArrayKeyExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var key *phpv.ZVal
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &key, &array)
	if err != nil {
		return nil, err
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
		for ; iter.Valid(ctx); iter.Next(ctx) {
			key, err := iter.Key(ctx)
			if err != nil {
				return nil, err
			}

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

	for k, v := range array.Iterate(ctx) {
		switch v.GetType() {
		case phpv.ZtInt, phpv.ZtString:
		default:
			ctx.Warn("Can only flip STRING and INTEGER values!")
			continue
		}

		result.OffsetSet(ctx, v, k)
	}

	return result.ZVal(), nil
}

// > func array array_filter ( array $array [, callable $callback [, int $flag = 0 ]] )
func fncArrayFilter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	var callback phpv.Callable
	var flagArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &array, &callback, &flagArg)
	if err != nil {
		return nil, err
	}

	var flag phpv.ZInt = 0
	if flagArg != nil {
		flag = *flagArg
	}

	result := phpv.NewZArray()

	switch flag {
	case ARRAY_FILTER_USE_BOTH:
		callbackArgs := make([]*phpv.ZVal, 2)
		for k, v := range array.Iterate(ctx) {
			callbackArgs[0] = v
			callbackArgs[1] = k
			ok, err := callback.Call(ctx, callbackArgs)
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
			ok, err := callback.Call(ctx, callbackArgs)
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
			ok, err := callback.Call(ctx, []*phpv.ZVal{v})
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
	var array *phpv.ZArray
	var callback phpv.Callable
	var userdata **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &userdata)
	if err != nil {
		return nil, err
	}

	iter := array.NewIterator()

	callbackArgs := make([]*phpv.ZVal, 2)
	if userdata != nil {
		callbackArgs = append(callbackArgs, *userdata)
	}

	// TODO: error if len(callbackArgs) is more than callback expects

	for ; iter.Valid(ctx); iter.Next(ctx) {
		val, err := iter.Current(ctx)
		if err != nil {
			return nil, err
		}
		key, err := iter.Key(ctx)
		if err != nil {
			return nil, err
		}

		callbackArgs[0] = val
		callbackArgs[1] = key

		callback.Call(ctx, callbackArgs)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func array array_map ( callable $callback , array $array1 [, array $... ] )
func fncArrayMap(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var callback phpv.Callable
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &callback, &array)
	if err != nil {
		return nil, err
	}

	maxLen := int(array.Count(ctx))
	arrays := []*phpv.ZArray{array}
	result := phpv.NewZArray()

	for i := 2; i < len(args); i++ {
		b, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		array := b.Value().(*phpv.ZArray)
		arrays = append(arrays, array)
		maxLen = max(maxLen, int(array.Count(ctx)))
	}

	for i := 0; i < maxLen; i++ {
		var args []*phpv.ZVal
		for _, arr := range arrays {
			val, err := arr.OffsetGet(ctx, phpv.ZInt(i))
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		val, err := callback.Call(ctx, args)
		if err != nil {
			return nil, err
		}
		err = result.OffsetSet(ctx, nil, val)
		if err != nil {
			return nil, err
		}
	}

	return result.ZVal(), nil
}

// > func array range (  callable $callback , array $array1 [, array $... ] )
func fncRange(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var start, end *phpv.ZVal
	var stepArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &start, &end, &stepArg)
	if err != nil {
		return nil, err
	}

	step := 1
	if stepArg != nil {
		step = int(*stepArg)
	}

	if step < 0 {
		step = -step
	}

	result := phpv.NewZArray()

	if start.GetType() == phpv.ZtString && end.GetType() == phpv.ZtString {
		s1 := []byte(start.AsString(ctx))
		s2 := []byte(end.AsString(ctx))

		// use only first character of the string

		if len(s1) == 0 || len(s2) == 0 {
			result.OffsetSet(ctx, nil, phpv.ZInt(0).ZVal())
		} else if s1[0] < s2[0] {
			for i := s1[0]; i <= s2[0]; i += byte(step) {
				c := string(rune(i))
				result.OffsetSet(ctx, nil, phpv.ZStr(c))
			}
		} else {
			for i := s1[0]; i >= s2[0]; i -= byte(step) {
				c := string(rune(i))
				result.OffsetSet(ctx, nil, phpv.ZStr(c))
			}
		}
	} else {
		n1 := int(start.AsInt(ctx))
		n2 := int(end.AsInt(ctx))
		if n1 < n2 {
			for i := n1; i <= n2; i += step {
				result.OffsetSet(ctx, nil, phpv.ZInt(i).ZVal())
			}
		} else {
			for i := n1; i >= n2; i -= step {
				result.OffsetSet(ctx, nil, phpv.ZInt(i).ZVal())
			}
		}
	}

	return result.ZVal(), nil
}

// > func mixed array_shift ( array &$array )
func fncArrayShift(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0].GetType() != phpv.ZtArray {
		ctx.Warnf("expects parameter 1 to be array, %s given", args[0].GetType())
		return phpv.ZNULL.ZVal(), nil
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var key *phpv.ZVal
	for key = range array.Iterate(ctx) {
		break
	}

	val, err := array.OffsetGet(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	err = array.OffsetUnset(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	return val.ZVal(), nil
}

// > func mixed array_pop ( array &$array )
func fncArrayPop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0].GetType() != phpv.ZtArray {
		ctx.Warnf("expects parameter 1 to be array, %s given", args[0].GetType())
		return phpv.ZNULL.ZVal(), nil
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var key *phpv.ZVal
	for key = range array.Iterate(ctx) {
		// iterate until last key
	}

	val, err := array.OffsetGet(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	err = array.OffsetUnset(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

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
		added := phpv.NewZArray()
		for k, v := range array.Iterate(ctx) {
			if ok, _ := added.OffsetExists(ctx, k); ok {
				continue
			}
			added.OffsetSet(ctx, v, phpv.ZTrue.ZVal())
			result.OffsetSet(ctx, k, v)
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
	var array *phpv.ZArray
	var offset phpv.ZInt
	var lengthArg *phpv.ZInt
	var preserveKeysArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &array, &offset, &lengthArg, &preserveKeysArg)
	if err != nil {
		return nil, ctx.Error(err)
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

	for key, val := range result.Iterate(ctx) {
		if key.GetType() != phpv.ZtInt {
			result.OffsetSet(ctx, key, val)
		}
	}

	if preserveKeys {
		for key := offset; key < end; key++ {
			val, _ := array.OffsetGet(ctx, key)
			result.OffsetSet(ctx, key, val)
		}
	} else {
		for key := offset; key < end; key++ {
			destKey := phpv.ZInt(key - offset)
			val, _ := array.OffsetGet(ctx, key)
			result.OffsetSet(ctx, destKey, val)
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
			if v.GetType() == needle.GetType() && v.Value() == needle.Value() {
				return k, nil
			}
		}
	} else {
		for k, v := range haystack.Iterate(ctx) {
			match := v.GetType() == needle.GetType() && v.Value() == needle.Value()
			match = match || v.String() == needle.String()
			if match {
				return k, nil
			}
		}
	}

	return phpv.ZFalse.ZVal(), nil
}

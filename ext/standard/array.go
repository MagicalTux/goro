package standard

import (
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/util"
)

// > const
const (
	CASE_LOWER phpv.ZInt = 0
	CASE_UPPER phpv.ZInt = 1
)

// > const
const (
	ARRAY_FILTER_USE_KEY  phpv.ZInt = 1
	ARRAY_FILTER_USE_BOTH phpv.ZInt = 2
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

	// array_flip behaves (maybe unexpectedly) in cases such as:
	//  array_flip([1=>'one','two', 3=>'three', 4, "five"=>5])
	//    equals to ['one'=>1, 'two'=>2, 'three'=>3, 4=>4, "five"=>5]
	//       not to ['one'=>1, 'two'=>0, 'three'=>3, 4=>2, "five"=>5]
	// so array_flip needs to know implicitly keyed values,
	// and the last maxKey.
	maxKey := phpv.ZInt(-1)

	it := array.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)

		switch v.GetType() {
		case phpv.ZtInt, phpv.ZtString:
		default:
			ctx.Warn("Can only flip STRING and INTEGER values!")
			continue
		}

		if k.GetType() == phpv.ZtInt {
			n := k.AsInt(ctx)
			if it.OmittedKey(ctx) {
				k = (maxKey + 1).ZVal()
				maxKey += 1
			} else {
				maxKey = max(maxKey, n)
			}
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
	var callbackArg core.Optional[phpv.Callable]
	var flagArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &array, &callbackArg, &flagArg)
	if err != nil {
		return nil, err
	}

	callback := callbackArg.GetOrDefault(&phpctx.ExtFunction{
		Func: arrayFilterDefaultCallback,
	})

	var flag phpv.ZInt = 0
	if flagArg.HasArg() {
		flag = flagArg.Get()
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
	var array core.Ref[*phpv.ZArray]
	var callback phpv.Callable
	var userdata **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &userdata)
	if err != nil {
		return nil, err
	}

	// TODO: error if len(callbackArgs) is more than callback expects

	callbackArgs := make([]*phpv.ZVal, 2)
	if userdata != nil {
		callbackArgs = append(callbackArgs, *userdata)
	}

	for k, v := range array.Get().Iterate(ctx) {
		callbackArgs[0] = v
		callbackArgs[1] = k
		callback.Call(ctx, callbackArgs)
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool array_walk_recursive ( array &$array , callable $callback [, mixed $userdata = NULL ] )
func fncArrayWalkRecursive(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	var callback phpv.Callable
	var userdata **phpv.ZVal
	_, err := core.Expand(ctx, args, &array, &callback, &userdata)
	if err != nil {
		return nil, err
	}

	// TODO: error if len(callbackArgs) is more than callback expects

	callbackArgs := make([]*phpv.ZVal, 2)
	if userdata != nil {
		callbackArgs = append(callbackArgs, *userdata)
	}

	var loop func(*phpv.ZArray)
	loop = func(array *phpv.ZArray) {
		for k, v := range array.Iterate(ctx) {
			if v.GetType() == phpv.ZtArray {
				loop(v.AsArray(ctx))
				continue
			}

			callbackArgs[0] = v
			callbackArgs[1] = k
			callback.Call(ctx, callbackArgs)
		}
	}

	loop(array.Get())

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

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var key *phpv.ZVal
	for key = range array.Get().Iterate(ctx) {
		break
	}

	val, err := array.Get().OffsetGet(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	err = array.Get().OffsetUnset(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	return val.ZVal(), nil
}

// > func int array_unshift ( array &$array [, mixed $... ] )
func fncArrayUnshift(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	it := array.Get().NewIterator()
	array.Get().Empty(ctx)

	index := 0
	for i := 1; i < len(args); i++ {
		array.Get().OffsetSet(ctx, phpv.ZInt(index), args[i])
		index++
	}
	for ; it.Valid(ctx); it.Next(ctx) {
		key, _ := it.Key(ctx)
		val, _ := it.Current(ctx)
		if key.GetType() == phpv.ZtInt {
			array.Get().OffsetSet(ctx, phpv.ZInt(index), val)
			index++
		} else {
			array.Get().OffsetSet(ctx, key, val)
		}
	}

	return phpv.ZInt(array.Get().Count(ctx)).ZVal(), nil
}

// > func int array_push ( array &$array [, mixed $... ] )
func fncArrayPush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	for i := 1; i < len(args); i++ {
		array.OffsetSet(ctx, nil, args[i])
	}

	return phpv.ZInt(array.Count(ctx)).ZVal(), nil
}

// > func mixed array_pop ( array &$array )
func fncArrayPop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 && args[0].GetType() != phpv.ZtArray {
		ctx.Warnf("expects parameter 1 to be array, %s given", args[0].GetType())
		return phpv.ZNULL.ZVal(), nil
	}

	var array core.Ref[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var key *phpv.ZVal
	for key = range array.Get().Iterate(ctx) {
		// iterate until last key
	}

	val, err := array.Get().OffsetGet(ctx, key)
	if err != nil {
		return nil, ctx.Error(err)
	}

	err = array.Get().OffsetUnset(ctx, key)
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

// > func mixed key ( array $array )
func fncArrayKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	key, err := array.MainIterator().Key(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	return key, nil
}

// > func mixed current ( array $array )
// > alias pos
func fncArrayCurrent(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.Error(err)
	}

	current, err := array.MainIterator().Current(ctx)
	if err != nil {
		return nil, ctx.Error(err)
	}
	return current, nil
}

// > func mixed next ( array &$array )
func fncArrayNext(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if size == 0 {
		ctx.Warn("Size parameter expected to be greater than 0")
		return phpv.ZNULL.ZVal(), nil
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
		if util.CtypeDigit(str) {
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

	columnKey, err := getArrayKeyValue(ctx, columnKeyArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var indexKey *phpv.ZVal
	if indexKeyArg.HasArg() {
		indexKey, err = getArrayKeyValue(ctx, indexKeyArg.Get())
		if err != nil {
			return nil, ctx.FuncError(err)
		}
	}

	// result is an array of { row[indexKey] : row[columnKey], ... }
	// where row = array[i]
	// if row[indexKey] doesn't exist or non-numeric, use maxIndex+1 as key

	result := phpv.NewZArray()
	var maxIndex phpv.ZInt = -1
	for _, item := range array.Iterate(ctx) {
		if item.GetType() != phpv.ZtArray {
			continue
		}
		row := item.AsArray(ctx)
		if exists, _ := row.OffsetExists(ctx, columnKey); !exists {
			continue
		}
		value, _ := row.OffsetGet(ctx, columnKey)

		var key *phpv.ZVal
		if indexKey != nil {
			if exists, _ := row.OffsetExists(ctx, indexKey); !exists {
				index := phpv.ZInt(maxIndex + 1)
				key = index.ZVal()
				if index > maxIndex {
					maxIndex = index
				}
			} else {
				k, _ := row.OffsetGet(ctx, indexKey)
				if k.GetType() == phpv.ZtInt {
					index := k.AsInt(ctx)
					key = index.ZVal()
					if index > maxIndex {
						maxIndex = index
					}
				} else {
					key = k
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
			ctx.Warn("Can only count STRING and INTEGER values!")
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

	result := phpv.NewZArray()
	for i := startIndex; i < startIndex+num; i++ {
		result.OffsetSet(ctx, phpv.ZInt(i), fillValue)
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

	result := phpv.NewZArray()
	for _, v := range array.Iterate(ctx) {
		result.OffsetSet(ctx, v, fillValue)
	}
	return result.ZVal(), nil
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

func arrayRecursiveMerge(ctx phpv.Context, result, array *phpv.ZArray) {
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

			arrayRecursiveMerge(ctx, array, v.AsArray(ctx))
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

func arrayRecursiveReplace(ctx phpv.Context, result, array *phpv.ZArray) {
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

			arrayRecursiveReplace(ctx, array, v.AsArray(ctx))
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

	var result *phpv.ZArray
	if size < 0 {
		size = -size
		result = phpv.NewZArray()
		for i := 0; i < int(size)-int(array.Count(ctx)); i++ {
			result.OffsetSet(ctx, nil, padValue)
		}
		for k, v := range array.Iterate(ctx) {
			if k.GetType() == phpv.ZtInt {
				result.OffsetSet(ctx, nil, v)
			} else {
				result.OffsetSet(ctx, k, v)
			}
		}
	} else {
		result = array.Dup()
		for i := 0; i < int(size)-int(array.Count(ctx)); i++ {
			result.OffsetSet(ctx, nil, padValue)
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

	var product phpv.ZFloat = 1
	for _, v := range array.Iterate(ctx) {
		if v.GetType() == phpv.ZtArray {
			continue
		}

		product *= v.AsFloat(ctx)
	}

	return product.ZVal(), nil
}

// > func number array_sum ( array $array )
func fncArraySum(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var sum phpv.ZFloat = 0
	for _, v := range array.Iterate(ctx) {
		if v.GetType() == phpv.ZtArray {
			continue
		}

		sum += v.AsFloat(ctx)
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
		ctx.Warn("Array is empty")
		return nil, nil
	}

	// TODO: use Mersenne Twister RNG for maximum compatibility

	num := core.Deref(numArg, 1)

	if num == 1 {
		i := rand.IntN(int(array.Count(ctx)))
		return array.OffsetKeyAt(ctx, i)
	}

	result := phpv.NewZArray()
	indices := rand.Perm(int(array.Count(ctx)))[:num]

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
		accumulator, err = callback.Call(ctx, cbArgs)
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
	prefix := core.Deref(prefixArgs, "")

	switch flags {
	case EXTR_PREFIX_SAME, EXTR_PREFIX_ALL, EXTR_PREFIX_INVALID, EXTR_PREFIX_IF_EXISTS:
		if prefix == "" {
			return nil, ctx.FuncErrorf("specified extract type requires the prefix parameter")
		}
	}

	// TODO: handle EXTR_REFS
	flags &= ^EXTR_REFS

	for k, v := range array.Get().Iterate(ctx) {
		alreadyDefined, _ := parentCtx.OffsetExists(ctx, k)

		var varName phpv.ZString = k.AsString(ctx)

		if !containsInvalidChar(string(varName)) {
			continue
		}

		invalidVarName := k.GetType() == phpv.ZtInt
		if !invalidVarName && !regexp.MustCompile(`^[a-zA-Z_]`).MatchString(string(varName)) {
			invalidVarName = true
		}

		switch flags {
		case EXTR_OVERWRITE:
			parentCtx.OffsetSet(parentCtx, k, v)
		case EXTR_SKIP:
			if !alreadyDefined {
				parentCtx.OffsetSet(parentCtx, k, v)
			}
		case EXTR_PREFIX_SAME:
			if alreadyDefined {
				prefixed := prefix + "_" + k.AsString(ctx)
				parentCtx.OffsetSet(parentCtx, prefixed, v)
			} else {
				parentCtx.OffsetSet(parentCtx, varName, v)
			}
		case EXTR_PREFIX_ALL:
			prefixed := prefix + "_" + k.AsString(ctx)
			parentCtx.OffsetSet(parentCtx, prefixed, v)
		case EXTR_PREFIX_INVALID:
			if invalidVarName {
				prefixed := prefix + "_" + k.AsString(ctx)
				parentCtx.OffsetSet(parentCtx, prefixed, v)
			} else {
				parentCtx.OffsetSet(parentCtx, varName, v)
			}
		case EXTR_IF_EXISTS:
			if alreadyDefined {
				parentCtx.OffsetSet(parentCtx, k, v)
			}
		case EXTR_PREFIX_IF_EXISTS:
			if alreadyDefined {
				prefixed := prefix + "_" + varName
				parentCtx.OffsetSet(parentCtx, prefixed, v)
			}
		}
	}

	return nil, nil
}

// > func array compact ( mixed $varname1 [, mixed $... ] )
func fncArrayCompact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		ctx.Warn("expects at least 1 parameter, 0 given")
		return nil, nil
	}
	parentCtx := ctx.Parent(1)
	result := phpv.NewZArray()
	for _, v := range args {
		err := arrayRecursiveCompact(parentCtx, result, v)
		if err != nil {
			return nil, ctx.Error(err)
		}
	}
	return result.ZVal(), nil
}

func arrayRecursiveCompact(ctx phpv.Context, result *phpv.ZArray, varName *phpv.ZVal) error {
	switch varName.GetType() {
	case phpv.ZtString:
		if ok, _ := ctx.OffsetExists(ctx, varName); ok {
			value, err := ctx.OffsetGet(ctx, varName)
			if err != nil {
				return err
			}
			result.OffsetSet(ctx, varName, value)
		}
	case phpv.ZtArray:
		for _, varName := range varName.AsArray(ctx).Iterate(ctx) {
			err := arrayRecursiveCompact(ctx, result, varName)
			if err != nil {
				return err
			}
		}
	default:
		// ignore other types
	}

	return nil
}

func containsInvalidChar(s string) bool {
	if s == "" {
		return false
	}

	buf := bytes.NewBufferString(s)

	for {
		c, err := buf.ReadByte()
		if err == io.EOF {
			break
		}
		switch {
		case
			'a' <= c && c <= 'z',
			'A' <= c && c <= 'Z',
			'0' <= c && c <= '9',
			c == '_',
			0x7f <= c:

		default:
			return false
		}
	}

	return true
}

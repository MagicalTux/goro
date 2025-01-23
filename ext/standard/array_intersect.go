package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

type containsEntryArgs struct {
	KeyEquals func(a, b *phpv.ZVal) bool
	ValEquals func(a, b *phpv.ZVal) bool
}

func arrayContainsEntry(ctx phpv.Context, array *phpv.ZArray, key, val *phpv.ZVal, args containsEntryArgs) bool {
	for k, v := range array.Iterate(ctx) {
		foundKey := true
		foundVal := true
		if args.KeyEquals != nil {
			foundKey = args.KeyEquals(key, k)
		}
		if args.ValEquals != nil {
			foundVal = args.ValEquals(val, v)
		}
		if foundKey && foundVal {
			return true
		}
	}
	return false
}

func expandArrayArgs(ctx phpv.Context, args []*phpv.ZVal) ([]*phpv.ZArray, error) {
	var result []*phpv.ZArray
	for i := 0; i < len(args); i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		result = append(result, val.AsArray(ctx))
	}
	return result, nil
}

// > func array array_intersect ( array $array2 , array $array2 [, array $... ] )
func fncArrayIntersect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Warn("at least 2 parameters are required, %d given", len(args))
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:])
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	result := phpv.NewZArray()

	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found, _ := arr.OffsetContains(ctx, v1)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_uintersect (array $array1 , array $array2 [, array $... ], callable $value_compare_func)
func fncArrayUIntersect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 3 {
		return nil, ctx.Warn("at least 3 parameters are required, %d given", len(args))
	}

	valueCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		return nil, ctx.Warn("expects parameter 3 to be a valid callback, no array or string given")
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:len(args)-1])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	containsArgs := containsEntryArgs{
		ValEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, valueCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
	}

	result := phpv.NewZArray()
	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_uintersect_assoc (array $array1 , array $array2 [, array $... ], callable $value_compare_func)
func fncArrayUIntersectAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 3 {
		return nil, ctx.Warn("at least 3 parameters are required, %d given", len(args))
	}

	valueCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		return nil, ctx.Warn("expects parameter 3 to be a valid callback, no array or string given")
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:len(args)-1])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	containsArgs := containsEntryArgs{
		KeyEquals: func(a, b *phpv.ZVal) bool {
			ok, _ := phpv.StrictEquals(ctx, a, b)
			return ok
		},
		ValEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, valueCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
	}

	result := phpv.NewZArray()
	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_uintersect_uassoc ( array $array1 , array $array2 [, array $... ], callable $value_compare_func , callable $key_compare_func )
func fncArrayUIntersectUAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 4 {
		return nil, ctx.Warn("at least 4 parameters are required, %d given", len(args))
	}

	valueCompare, err := core.SpawnCallable(ctx, args[len(args)-2])
	if err != nil {
		return nil, ctx.Warn("expects parameter %d to be a valid callback, no array or string given", len(args)-2)
	}

	keyCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		return nil, ctx.Warn("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:len(args)-2])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	containsArgs := containsEntryArgs{
		KeyEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, keyCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
		ValEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, valueCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
	}

	result := phpv.NewZArray()
	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_intersect_assoc ( array $array1 , array $array2 [, array $... ] )
func fncArrayIntersectAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Warn("at least 2 parameters are required, %d given", len(args))
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	containsArgs := containsEntryArgs{
		KeyEquals: func(a, b *phpv.ZVal) bool {
			ok, _ := phpv.StrictEquals(ctx, a, b)
			return ok
		},
		ValEquals: func(a, b *phpv.ZVal) bool {
			ok, _ := phpv.Equals(ctx, a, b)
			return ok
		},
	}

	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_intersect_uassoc ( array $array1 , array $array2 [, array $... ], callable $key_compare_func )
func fncArrayIntersectUAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 3 {
		return nil, ctx.Warn("at least 3 parameters are required, %d given", len(args))
	}

	keyCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		return nil, ctx.Warn("expects parameter 3 to be a valid callback, no array or string given")
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:len(args)-1])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	containsArgs := containsEntryArgs{
		KeyEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, keyCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
		ValEquals: func(a, b *phpv.ZVal) bool {
			return a.AsString(ctx) == b.AsString(ctx)
		},
	}

	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_intersect_key ( array $array1 , array $array2 [, array $... ] )
func fncArrayIntersectKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found, _ := arr.OffsetExists(ctx, k1)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

// > func array array_intersect_ukey ( array $array1 , array $array2 [, array $... ], callable $key_compare_func )
func fncArrayIntersectUKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 3 {
		return nil, ctx.Warn("at least 3 parameters are required, %d given", len(args))
	}

	keyCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		return nil, ctx.Warn("expects parameter 3 to be a valid callback, no array or string given")
	}

	otherArrays, err := expandArrayArgs(ctx, args[1:])
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := phpv.NewZArray()
	containsArgs := containsEntryArgs{
		KeyEquals: func(a, b *phpv.ZVal) bool {
			ret, _ := ctx.CallZVal(ctx, keyCompare, []*phpv.ZVal{a, b})
			return ret.AsInt(ctx) == 0
		},
	}

	for k1, v1 := range array.Iterate(ctx) {
		foundInAll := core.Every(otherArrays, func(arr *phpv.ZArray) bool {
			found := arrayContainsEntry(ctx, arr, k1, v1, containsArgs)
			return found
		})
		if foundInAll {
			result.OffsetSet(ctx, k1, v1)
		}
	}

	return result.ZVal(), nil
}

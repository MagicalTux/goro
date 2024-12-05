package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func array array_intersect ( array $array2 , array $array2 [, array $... ] )
func fncArrayIntersect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		ctx.Warnf("at least 2 parameters are required, %d given", len(args))
		return nil, nil
	}

	result := phpv.NewZArray()
	for i := 1; i < len(args); i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arr := val.AsArray(ctx)
		for k1, v1 := range array.Iterate(ctx) {
			if ok, _ := result.OffsetExists(ctx, k1); ok {
				continue
			}
			for _, v2 := range arr.Iterate(ctx) {
				if ok, _ := core.StrictEquals(ctx, v1, v2); !ok {
					continue
				}
				result.OffsetSet(ctx, k1, v1)
			}
		}
	}

	return result.ZVal(), nil
}

// > func array array_intersect_assoc ( array $array1 , array $array2 [, array $... ] )
func fncArrayIntersectAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		ctx.Warnf("at least 2 parameters are required, %d given", len(args))
		return nil, nil
	}

	result := phpv.NewZArray()
	for i := 1; i < len(args); i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arr := val.AsArray(ctx)
		for k1, v1 := range array.Iterate(ctx) {
			if ok, _ := result.OffsetExists(ctx, k1); ok {
				continue
			}
			for k2, v2 := range arr.Iterate(ctx) {
				if ok, _ := core.StrictEquals(ctx, k1, k2); !ok {
					continue
				}
				if ok, _ := core.StrictEquals(ctx, v1, v2); !ok {
					continue
				}
				result.OffsetSet(ctx, k1, v1)
			}
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
		ctx.Warnf("at least 3 parameters are required, %d given", len(args))
		return nil, nil
	}

	keyCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		ctx.Warnf("expects parameter 3 to be a valid callback, no array or string given")
		return nil, nil
	}

	result := phpv.NewZArray()

	compareArgs := make([]*phpv.ZVal, 2)
	for i := 1; i < len(args)-1; i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arr := val.AsArray(ctx)
		for k1, v1 := range array.Iterate(ctx) {
			if ok, _ := result.OffsetExists(ctx, k1); ok {
				continue
			}
			for k2, v2 := range arr.Iterate(ctx) {

				compareArgs[0] = k1
				compareArgs[1] = k2
				ret, err := keyCompare.Call(ctx, compareArgs)
				if err != nil {
					return nil, ctx.FuncError(err)
				}
				if v2.AsString(ctx) != v1.AsString(ctx) {
					continue
				}
				if ret.AsInt(ctx) != 0 {
					continue
				}

				result.OffsetSet(ctx, k1, v1)
			}
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

	if len(args) < 2 {
		ctx.Warnf("at least 2 parameters are required, %d given", len(args))
		return nil, nil
	}

	result := phpv.NewZArray()
	for i := 1; i < len(args); i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arr := val.AsArray(ctx)
		for k1, v1 := range array.Iterate(ctx) {
			if ok, _ := result.OffsetExists(ctx, k1); ok {
				continue
			}
			for k2 := range arr.Iterate(ctx) {

				if ok, _ := core.StrictEquals(ctx, k2, k1); ok {
					result.OffsetSet(ctx, k1, v1)
				}
			}
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
		ctx.Warnf("at least 3 parameters are required, %d given", len(args))
		return nil, nil
	}

	keyCompare, err := core.SpawnCallable(ctx, args[len(args)-1])
	if err != nil {
		ctx.Warnf("expects parameter 3 to be a valid callback, no array or string given")
		return nil, nil
	}

	result := phpv.NewZArray()

	compareArgs := make([]*phpv.ZVal, 2)
	for i := 1; i < len(args)-1; i++ {
		val, err := args[i].As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, ctx.FuncError(err)
		}
		arr := val.AsArray(ctx)
		for k1, v1 := range array.Iterate(ctx) {
			if ok, _ := result.OffsetExists(ctx, k1); ok {
				continue
			}
			for k2 := range arr.Iterate(ctx) {
				compareArgs[0] = k1
				compareArgs[1] = k2
				ret, err := keyCompare.Call(ctx, compareArgs)
				if err != nil {
					return nil, ctx.FuncError(err)
				}
				if ret.AsInt(ctx) != 0 {
					continue
				}

				result.OffsetSet(ctx, k1, v1)
			}
		}
	}

	return result.ZVal(), nil
}


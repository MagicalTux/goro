package standard

import (
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type arrayDiffFn func(k1, v1, k2, v2 *phpv.ZVal) (bool, error)

func arrayDiff(ctx phpv.Context, array *phpv.ZArray, args []*phpv.ZVal, shouldRemove arrayDiffFn) error {
	return arrayDiffNamed(ctx, "", 2, array, args, shouldRemove)
}

func arrayDiffNamed(ctx phpv.Context, funcName string, argOffset int, array *phpv.ZArray, args []*phpv.ZVal, shouldRemove arrayDiffFn) error {
	// array_diff($array, $xs, $ys, $zs)
	// Basically, what array_diff does is to remove entries in $array
	// that is contained by any of the other arrays $xs, $ys, etc.
	// The keys in $array will not be shifted/modified, only for deletion
	for i := 0; i < len(args); i++ {
		if args[i].GetType() != phpv.ZtArray {
			if funcName != "" {
				return phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("%s(): Argument #%d must be of type array, %s given", funcName, i+argOffset, args[i].GetType().TypeName()))
			}
			return phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Argument #%d must be of type array, %s given", i+argOffset, args[i].GetType().TypeName()))
		}
		array2 := args[i].AsArray(ctx)

		for k1, v1 := range array.Iterate(ctx) {
			for k2, v2 := range array2.Iterate(ctx) {
				if ok, err := shouldRemove(k2, v2, k1, v1); err != nil {
					return err
				} else if ok {
					array.OffsetUnset(ctx, k1)
				}
			}
		}
	}

	return nil
}

// > func array array_diff ( array $array1 , array $array2 [, array $... ] )
func fncArrayDiff(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) >= 1 && args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("array_diff(): Argument #1 ($array) must be of type array, %s given", args[0].GetType().TypeName()))
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := array.Dup()
	err = arrayDiffNamed(ctx, "array_diff", 2, result, args[1:], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		return v1.AsString(ctx) == v2.AsString(ctx), nil
	})
	if err != nil {
		return nil, err
	}

	return result.ZVal(), nil
}

// > func array array_udiff ( array $array1 , array $array2 [, array $... ], callable $value_compare_func )
func fncArrayUDiff(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Errorf("at least 2 parameters are required, %d given", len(args))
	}

	lastArg := args[len(args)-1]
	switch lastArg.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}

	valueCompareFunc, err := core.SpawnCallable(ctx, lastArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	funcArgs := make([]*phpv.ZVal, 2)
	result := array.Dup()
	err = arrayDiff(ctx, result, args[1:len(args)-1], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		funcArgs[0] = v1
		funcArgs[1] = v2
		ret, err := ctx.CallZValInternal(ctx, valueCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}

		return ret.AsInt(ctx) == 0, nil
	})
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func array array_udiff_assoc ( array $array1 , array $array2 [, array $... ], callable $value_compare_func )
func fncArrayUDiffAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Errorf("at least 2 parameters are required, %d given", len(args))
	}

	lastArg := args[len(args)-1]
	switch lastArg.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}

	valueCompareFunc, err := core.SpawnCallable(ctx, lastArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	funcArgs := make([]*phpv.ZVal, 2)
	result := array.Dup()
	err = arrayDiff(ctx, result, args[1:len(args)-1], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		if k1.AsString(ctx) != k2.AsString(ctx) {
			return false, nil
		}
		funcArgs[0] = v1
		funcArgs[1] = v2
		ret, err := ctx.CallZValInternal(ctx, valueCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}

		return ret.AsInt(ctx) == 0, nil
	})
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func array array_udiff_uassoc ( array $array1 , array $array2 [, array $... ], callable $value_compare_func , callable $key_compare_func )
func fncArrayUDiffUAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 3 {
		return nil, ctx.Errorf("at least 3 parameters are required, %d given", len(args))
	}

	lastArg2 := args[len(args)-2]
	lastArg1 := args[len(args)-1]

	switch lastArg1.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}
	switch lastArg2.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-2)
	}

	valueCompareFunc, err := core.SpawnCallable(ctx, lastArg2)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	keyCompareFunc, err := core.SpawnCallable(ctx, lastArg1)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	funcArgs := make([]*phpv.ZVal, 2)
	result := array.Dup()
	err = arrayDiff(ctx, result, args[1:len(args)-2], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		funcArgs[0] = k1
		funcArgs[1] = k2
		ret, err := ctx.CallZValInternal(ctx, keyCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}
		if ret.AsInt(ctx) != 0 {
			return false, nil
		}

		funcArgs[0] = v1
		funcArgs[1] = v2
		ret, err = ctx.CallZValInternal(ctx, valueCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}
		if ret.AsInt(ctx) != 0 {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func array array_diff_ukey ( array $array1 , array $array2 [, array $... ], callable $key_compare_func )
func fncArrayDiffUKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Errorf("at least 2 parameters are required, %d given", len(args))
	}

	lastArg := args[len(args)-1]
	switch lastArg.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}

	valueCompareFunc, err := core.SpawnCallable(ctx, lastArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	funcArgs := make([]*phpv.ZVal, 2)
	result := array.Dup()
	err = arrayDiff(ctx, result, args[1:len(args)-1], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		funcArgs[0] = k1
		funcArgs[1] = k2
		ret, err := ctx.CallZValInternal(ctx, valueCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}

		return ret.AsInt(ctx) == 0, nil
	})
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func array array_diff_uassoc ( array $array1 , array $array2 [, array $... ], callable $value_compare_func )
func fncArrayDiffUAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(args) < 2 {
		return nil, ctx.Errorf("at least 2 parameters are required, %d given", len(args))
	}

	lastArg := args[len(args)-1]
	switch lastArg.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		return nil, ctx.FuncErrorf("expects parameter %d to be a valid callback, no array or string given", len(args)-1)
	}

	valueCompareFunc, err := core.SpawnCallable(ctx, lastArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	funcArgs := make([]*phpv.ZVal, 2)
	result := array.Dup()
	err = arrayDiff(ctx, result, args[1:len(args)-1], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		if v1.AsString(ctx) != v2.AsString(ctx) {
			return false, nil
		}

		funcArgs[0] = k1
		funcArgs[1] = k2
		ret, err := ctx.CallZValInternal(ctx, valueCompareFunc, funcArgs)
		if err != nil {
			return false, err
		}

		return ret.AsInt(ctx) == 0, nil
	})
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return result.ZVal(), nil
}

// > func array array_diff_key ( array $array1 , array $array2 [, array $... ] )
func fncArrayDiffKey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) >= 1 && args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("array_diff_key(): Argument #1 ($array) must be of type array, %s given", args[0].GetType().TypeName()))
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := array.Dup()
	err = arrayDiffNamed(ctx, "array_diff_key", 2, result, args[1:], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		return k1.AsString(ctx) == k2.AsString(ctx), nil
	})
	if err != nil {
		return nil, err
	}

	return result.ZVal(), nil
}

// > func array array_diff_assoc ( array $array1 , array $array2 [, array $... ] )
func fncArrayDiffAssoc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) >= 1 && args[0].GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("array_diff_assoc(): Argument #1 ($array) must be of type array, %s given", args[0].GetType().TypeName()))
	}

	var array *phpv.ZArray
	_, err := core.Expand(ctx, args, &array)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := array.Dup()
	err = arrayDiffNamed(ctx, "array_diff_assoc", 2, result, args[1:], func(k1, v1, k2, v2 *phpv.ZVal) (bool, error) {
		if k1.AsString(ctx) != k2.AsString(ctx) {
			return false, nil
		}
		if v1.AsString(ctx) != v2.AsString(ctx) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return result.ZVal(), nil
}

package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

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

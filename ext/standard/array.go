package standard

import "github.com/MagicalTux/goro/core"

//> func array array_merge ( array $array1 [, array $... ] )
func fncArrayMerge(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var a *core.ZArray
	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}
	a = a.Dup() // make sure we do a copy of array

	for i := 1; i < len(args); i++ {
		b, err := args[i].As(ctx, core.ZtArray)
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

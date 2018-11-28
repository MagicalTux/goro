package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func array array_merge ( array $array1 [, array $... ] )
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

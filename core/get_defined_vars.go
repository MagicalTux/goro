package core

import (
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func array get_defined_vars ( void )
func fncGetDefinedVars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	// get_defined_vars() returns variables from the calling scope, not from
	// within the built-in function call frame itself.
	parentCtx := ctx.Parent(1)

	// Iterate the calling scope's variable table
	it := parentCtx.NewIterator()
	for it.Valid(ctx) {
		key, _ := it.Key(ctx)
		val, _ := it.Current(ctx)
		if key != nil && val != nil {
			result.OffsetSet(ctx, key.Dup(), val.Dup())
		}
		it.Next(ctx)
	}

	return result.ZVal(), nil
}

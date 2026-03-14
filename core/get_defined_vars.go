package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func array get_defined_vars ( void )
func fncGetDefinedVars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()

	// Iterate the current scope's variable table
	it := ctx.NewIterator()
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

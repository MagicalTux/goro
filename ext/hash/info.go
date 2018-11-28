package hash

import (
	"github.com/MagicalTux/goro/core/phpv"
)

//> func array hash_algos ( void )
func fncHashAlgos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	a := phpv.NewZArray()

	for n := range algos {
		a.OffsetSet(ctx, nil, n.ZVal())
	}
	return a.ZVal(), nil
}

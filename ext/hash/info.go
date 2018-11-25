package hash

import "github.com/MagicalTux/gophp/core"

//> func array hash_algos ( void )
func fncHashAlgos(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	a := core.NewZArray()

	for n := range algos {
		a.OffsetSet(ctx, nil, n.ZVal())
	}
	return a.ZVal(), nil
}

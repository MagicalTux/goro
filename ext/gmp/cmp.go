package gmp

import "github.com/MagicalTux/gophp/core"

//> func int gmp_cmp ( GMP $a , GMP $b )
func gmpCmp(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var a, b *core.ZVal
	_, err := core.Expand(ctx, args, &a, &b)
	if err != nil {
		return nil, err
	}

	ia, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}
	ib, err := readInt(ctx, b)
	if err != nil {
		return nil, err
	}

	return core.ZInt(ia.Cmp(ib)).ZVal(), nil
}

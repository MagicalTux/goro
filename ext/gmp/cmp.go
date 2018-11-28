package gmp

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func int gmp_cmp ( GMP $a , GMP $b )
func gmpCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a, b *phpv.ZVal
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

	return phpv.ZInt(ia.Cmp(ib)).ZVal(), nil
}

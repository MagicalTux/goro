package gmp

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func int gmp_sign ( GMP $a )
func gmpSign(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, a, "gmp_sign", 1, "num")
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(i.Sign()).ZVal(), nil
}

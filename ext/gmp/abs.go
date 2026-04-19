package gmp

import (
	"math/big"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func GMP gmp_abs ( GMP $a )
func gmpAbs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal

	_, err := core.Expand(ctx, args, &gmpnum)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, gmpnum, "gmp_abs", 1, "num")
	if err != nil {
		return nil, err
	}

	r := &big.Int{}
	r.Abs(i)

	return returnInt(ctx, r)
}

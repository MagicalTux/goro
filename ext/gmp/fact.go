package gmp

import (
	"math/big"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func GMP gmp_fact ( int $a )
func gmpFact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, a, "gmp_fact", 1, "num")
	if err != nil {
		return nil, err
	}

	if i.Sign() < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_fact(): Argument #1 ($num) must be greater than or equal to 0")
	}

	r := &big.Int{}
	r.MulRange(1, i.Int64())

	return returnInt(ctx, r)
}

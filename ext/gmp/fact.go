package gmp

import (
	"errors"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_fact ( int $a )
func gmpFact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if i.Sign() < 0 {
		return nil, errors.New("Number has to be greater than or equal to 0")
	}

	r := &big.Int{}
	r.MulRange(1, i.Int64())

	return returnInt(ctx, r)
}

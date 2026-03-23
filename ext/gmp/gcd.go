package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_gcd ( GMP $a , GMP $b )
func gmpGcd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	r := &big.Int{}
	r.GCD(nil, nil, ia, ib)

	// GCD is always non-negative
	r.Abs(r)

	return returnInt(ctx, r)
}

// > func GMP gmp_lcm ( GMP $a , GMP $b )
func gmpLcm(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	// LCM(a, b) = |a * b| / GCD(a, b)
	// If either is zero, LCM is zero
	if ia.Sign() == 0 || ib.Sign() == 0 {
		return returnInt(ctx, big.NewInt(0))
	}

	gcd := &big.Int{}
	gcd.GCD(nil, nil, ia, ib)

	r := &big.Int{}
	r.Div(ia, gcd)
	r.Mul(r, ib)
	r.Abs(r)

	return returnInt(ctx, r)
}

// > func array gmp_gcdext ( GMP $a , GMP $b )
func gmpGcdext(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	g := &big.Int{}
	s := &big.Int{}
	t := &big.Int{}
	g.GCD(s, t, ia, ib)

	gv, err := returnInt(ctx, g)
	if err != nil {
		return nil, err
	}
	sv, err := returnInt(ctx, s)
	if err != nil {
		return nil, err
	}
	tv, err := returnInt(ctx, t)
	if err != nil {
		return nil, err
	}

	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZString("g").ZVal(), gv)
	arr.OffsetSet(ctx, phpv.ZString("s").ZVal(), sv)
	arr.OffsetSet(ctx, phpv.ZString("t").ZVal(), tv)

	return arr.ZVal(), nil
}

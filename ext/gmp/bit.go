package gmp

import (
	"math/big"

	"github.com/MagicalTux/gophp/core"
)

//> func void gmp_setbit ( GMP $a , int $index [, bool $bit_on = TRUE ] )
func gmpSetbit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	a := &core.ZObject{Class: GMP}
	var index core.ZInt
	var bitOn *core.ZBool

	_, err := core.Expand(ctx, args, &a, &index, &bitOn)
	if err != nil {
		return nil, err
	}
	i, ok := a.GetOpaque(GMP).(*big.Int)
	if !ok {
		i = &big.Int{}
	}

	b := uint(1)
	if bitOn != nil && !*bitOn {
		b = 0
	}

	r := &big.Int{}
	r.SetBit(i, int(index), b)

	a.SetOpaque(GMP, r)

	return nil, nil
}

//> func void gmp_clrbit ( GMP $a , int $index )
func gmpClrbit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	a := &core.ZObject{Class: GMP}
	var index core.ZInt

	_, err := core.Expand(ctx, args, &a, &index)
	if err != nil {
		return nil, err
	}
	i, ok := a.GetOpaque(GMP).(*big.Int)
	if !ok {
		i = &big.Int{}
	}

	r := &big.Int{}
	r.SetBit(i, int(index), 0)

	a.SetOpaque(GMP, r)

	return nil, nil
}

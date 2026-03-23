package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int gmp_popcount ( GMP $a )
func gmpPopcount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	// For negative numbers, PHP returns -1 (infinite set bits in two's complement)
	if i.Sign() < 0 {
		return phpv.ZInt(-1).ZVal(), nil
	}

	// Count set bits
	count := 0
	tmp := &big.Int{}
	tmp.Set(i)
	for tmp.Sign() > 0 {
		count += int(tmp.Bit(0))
		tmp.Rsh(tmp, 1)
	}

	return phpv.ZInt(count).ZVal(), nil
}

// > func bool gmp_testbit ( GMP $a , int $index )
func gmpTestbit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var index phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &index)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if index < 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	if i.Bit(int(index)) != 0 {
		return phpv.ZTrue.ZVal(), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

// > func int gmp_hamdist ( GMP $a , GMP $b )
func gmpHamdist(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	// If either is negative, return -1 (like popcount for negative)
	if ia.Sign() < 0 || ib.Sign() < 0 {
		return phpv.ZInt(-1).ZVal(), nil
	}

	// Hamming distance = popcount(a XOR b)
	xor := new(big.Int).Xor(ia, ib)

	count := 0
	for xor.Sign() > 0 {
		count += int(xor.Bit(0))
		xor.Rsh(xor, 1)
	}

	return phpv.ZInt(count).ZVal(), nil
}

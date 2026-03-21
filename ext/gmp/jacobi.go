package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int gmp_jacobi ( GMP $num1 , GMP $num2 )
func gmpJacobi(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	result := big.Jacobi(ia, ib)
	return phpv.ZInt(result).ZVal(), nil
}

// > func int gmp_legendre ( GMP $num1 , GMP $num2 )
// Legendre symbol is the same as the Jacobi symbol when num2 is an odd prime
func gmpLegendre(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	result := big.Jacobi(ia, ib)
	return phpv.ZInt(result).ZVal(), nil
}

// > func int gmp_kronecker ( GMP $num1 , GMP $num2 )
// Kronecker symbol is a generalization of the Jacobi symbol
func gmpKronecker(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	// Kronecker symbol extends Jacobi to even numbers and negative numbers.
	// For the general case, we implement it manually.
	result := kroneckerSymbol(ia, ib)
	return phpv.ZInt(result).ZVal(), nil
}

// kroneckerSymbol computes the Kronecker symbol (a/n) which generalizes
// the Jacobi symbol to all integers n.
func kroneckerSymbol(a, n *big.Int) int {
	zero := big.NewInt(0)
	one := big.NewInt(1)
	_ = big.NewInt(2) // two - reserved for future use

	// Handle n == 0
	if n.Sign() == 0 {
		absA := new(big.Int).Abs(a)
		if absA.Cmp(one) == 0 {
			return 1
		}
		return 0
	}

	// Handle n == 1
	if n.Cmp(one) == 0 {
		return 1
	}

	// Handle n == -1
	minusOne := big.NewInt(-1)
	if n.Cmp(minusOne) == 0 {
		if a.Sign() < 0 {
			return -1
		}
		return 1
	}

	// Factor out the sign of n: (a/n) = (a/|n|) * (a/-1) if n < 0
	result := 1
	nn := new(big.Int).Set(n)
	if nn.Sign() < 0 {
		nn.Neg(nn)
		if a.Sign() < 0 {
			result = -result
		}
	}

	// Factor out powers of 2 from nn
	// (a/2) = 0 if a is even, (-1)^((a^2-1)/8) if a is odd
	for nn.Bit(0) == 0 {
		nn.Rsh(nn, 1)
		aMod8 := new(big.Int).And(a, big.NewInt(7)).Int64()
		if aMod8 == 3 || aMod8 == 5 {
			result = -result
		}
		// Handle negative a mod 8
		if a.Sign() < 0 {
			absAMod8 := new(big.Int).Mod(new(big.Int).Abs(a), big.NewInt(8)).Int64()
			_ = absAMod8
		}
	}

	if nn.Cmp(one) == 0 {
		return result
	}

	// Now nn is odd and positive, use the Jacobi symbol
	aa := new(big.Int).Mod(a, nn)
	if aa.Sign() < 0 {
		aa.Add(aa, nn)
	}

	// Handle a == 0
	if aa.Cmp(zero) == 0 {
		return 0
	}

	j := big.Jacobi(aa, nn)
	return result * j
}

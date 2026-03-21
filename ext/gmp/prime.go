package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_nextprime ( GMP $a )
func gmpNextprime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	// Start from i+1 and find the next probable prime
	r := &big.Int{}
	r.Add(i, big.NewInt(1))

	// Ensure we start from at least 2
	if r.Cmp(big.NewInt(2)) < 0 {
		r.SetInt64(2)
	}

	for !r.ProbablyPrime(25) {
		r.Add(r, big.NewInt(1))
	}

	return returnInt(ctx, r)
}

// > func int gmp_prob_prime ( GMP $a [, int $reps = 10 ] )
func gmpProbPrime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var reps *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &reps)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	n := 10
	if reps != nil {
		n = int(*reps)
		if n < 0 {
			n = 0
		}
	}

	// PHP returns:
	// 0 - definitely not prime
	// 1 - probably prime
	// 2 - definitely prime
	if i.Sign() <= 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	if n == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	if i.ProbablyPrime(n) {
		// For small known primes we can return 2 (definitely prime)
		// For larger numbers return 1 (probably prime)
		if i.BitLen() <= 63 {
			v := i.Int64()
			if v <= 1 {
				return phpv.ZInt(0).ZVal(), nil
			}
			if v <= 3 {
				return phpv.ZInt(2).ZVal(), nil
			}
			// Use a deterministic check for small numbers
			return phpv.ZInt(2).ZVal(), nil
		}
		return phpv.ZInt(1).ZVal(), nil
	}

	return phpv.ZInt(0).ZVal(), nil
}

// > func bool gmp_perfect_square ( GMP $a )
func gmpPerfectSquare(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return phpv.ZFalse.ZVal(), nil
	}

	// Compute integer square root and check if sqrt*sqrt == i
	s := &big.Int{}
	s.Sqrt(i)

	sq := &big.Int{}
	sq.Mul(s, s)

	if sq.Cmp(i) == 0 {
		return phpv.ZTrue.ZVal(), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

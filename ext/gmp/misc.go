package gmp

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool gmp_perfect_power ( GMP $num )
func gmpPerfectPower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if isPerfectPower(i) {
		return phpv.ZTrue.ZVal(), nil
	}
	return phpv.ZFalse.ZVal(), nil
}

// isPerfectPower checks if n = a^b for some integers a, b with b >= 2.
// 0, 1, and -1 are considered perfect powers.
func isPerfectPower(n *big.Int) bool {
	zero := big.NewInt(0)
	one := big.NewInt(1)
	minusOne := big.NewInt(-1)

	// 0, 1, -1 are perfect powers
	if n.Cmp(zero) == 0 || n.Cmp(one) == 0 || n.Cmp(minusOne) == 0 {
		return true
	}

	absN := new(big.Int).Abs(n)
	negative := n.Sign() < 0

	// Try exponents from 2 upward
	// We only need to try prime exponents up to log2(|n|)
	maxExp := absN.BitLen()
	if maxExp > 1000 {
		maxExp = 1000
	}

	primes := []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61}

	for _, p := range primes {
		if p > maxExp {
			break
		}
		// For negative numbers, only odd exponents make sense
		if negative && p%2 == 0 {
			continue
		}

		root := nthRoot(absN, p)
		if root != nil {
			// Verify: root^p == |n|
			result := new(big.Int).Exp(root, big.NewInt(int64(p)), nil)
			if result.Cmp(absN) == 0 {
				return true
			}
		}
	}

	return false
}

// nthRoot returns the integer nth root of x, or nil if it's not exact.
// Uses binary search.
func nthRoot(x *big.Int, n int) *big.Int {
	if x.Sign() == 0 {
		return big.NewInt(0)
	}

	// Initial guess using bit length
	bitLen := x.BitLen()
	guessBits := (bitLen + n - 1) / n
	if guessBits < 1 {
		guessBits = 1
	}

	// Newton's method for nth root
	nBig := big.NewInt(int64(n))
	nMinus1 := big.NewInt(int64(n - 1))

	// Start with a power-of-2 guess
	guess := new(big.Int).Lsh(big.NewInt(1), uint(guessBits))

	for i := 0; i < bitLen+10; i++ {
		// newGuess = ((n-1) * guess + x / guess^(n-1)) / n
		guessNm1 := new(big.Int).Exp(guess, nMinus1, nil)
		if guessNm1.Sign() == 0 {
			break
		}
		xDivGuessNm1 := new(big.Int).Div(x, guessNm1)
		newGuess := new(big.Int).Mul(nMinus1, guess)
		newGuess.Add(newGuess, xDivGuessNm1)
		newGuess.Div(newGuess, nBig)

		if newGuess.Cmp(guess) >= 0 {
			break
		}
		guess = newGuess
	}

	// Check guess and guess+1
	result := new(big.Int).Exp(guess, nBig, nil)
	if result.Cmp(x) == 0 {
		return guess
	}
	guess.Add(guess, big.NewInt(1))
	result.Exp(guess, nBig, nil)
	if result.Cmp(x) == 0 {
		return guess
	}

	return nil
}

// > func string gmp_export ( GMP $num [, int $word_size = 1 [, int $options = GMP_MSW_FIRST | GMP_BIG_ENDIAN ]] )
func gmpExport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var wordSize *phpv.ZInt
	var options *phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &wordSize, &options)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	ws := phpv.ZInt(1)
	if wordSize != nil {
		ws = *wordSize
	}

	if ws <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_export(): Argument #2 ($word_size) must be greater than or equal to 1")
	}

	b := i.Bytes()
	if len(b) == 0 {
		b = []byte{0}
	}

	// Check if word_size is too large
	if int64(ws) > int64(len(b)) && ws > 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_export(): Argument #2 ($word_size) is too large for argument #1 ($num)")
	}

	return phpv.ZString(b).ZVal(), nil
}

// > func GMP gmp_random_bits ( int $bits )
func gmpRandomBits(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var bits phpv.ZInt

	_, err := core.Expand(ctx, args, &bits)
	if err != nil {
		return nil, err
	}

	maxBits := int64(math.MaxInt32)
	if bits < 1 || int64(bits) > maxBits {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_random_bits(): Argument #1 ($bits) must be between 1 and %d", maxBits))
	}

	// Generate a random number with the given number of bits
	max := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	r, err2 := rand.Int(rand.Reader, max)
	if err2 != nil {
		return nil, err2
	}

	return returnInt(ctx, r)
}

// > func GMP gmp_random_range ( GMP $min , GMP $max )
func gmpRandomRange(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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

	if ia.Cmp(ib) > 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"gmp_random_range(): Argument #1 ($min) must be less than or equal to argument #2 ($max)")
	}

	// range = max - min + 1
	rangeVal := new(big.Int).Sub(ib, ia)
	rangeVal.Add(rangeVal, big.NewInt(1))

	r, err2 := rand.Int(rand.Reader, rangeVal)
	if err2 != nil {
		return nil, err2
	}

	r.Add(r, ia)

	return returnInt(ctx, r)
}

// > func int gmp_scan0 ( GMP $num , int $start )
func gmpScan0(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var start phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &start)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if start < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_scan0(): Argument #2 ($start) must be greater than or equal to 0"))
	}

	// Find the first 0 bit at or after position start
	for pos := int(start); ; pos++ {
		if i.Bit(pos) == 0 {
			return phpv.ZInt(pos).ZVal(), nil
		}
		// Safety limit to avoid infinite loop for -1 (all bits set)
		if pos > int(start)+i.BitLen()+64 {
			return phpv.ZInt(-1).ZVal(), nil
		}
	}
}

// > func int gmp_scan1 ( GMP $num , int $start )
func gmpScan1(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var start phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &start)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if start < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_scan1(): Argument #2 ($start) must be greater than or equal to 0"))
	}

	// For zero, there are no set bits
	if i.Sign() == 0 {
		return phpv.ZInt(-1).ZVal(), nil
	}

	// Find the first 1 bit at or after position start
	maxBits := i.BitLen() + 1
	if i.Sign() < 0 {
		// For negative numbers in two's complement, bits extend infinitely
		maxBits = int(start) + i.BitLen() + 64
	}
	for pos := int(start); pos < maxBits; pos++ {
		if i.Bit(pos) != 0 {
			return phpv.ZInt(pos).ZVal(), nil
		}
	}

	return phpv.ZInt(-1).ZVal(), nil
}

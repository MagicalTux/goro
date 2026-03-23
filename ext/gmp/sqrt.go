package gmp

import (
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func GMP gmp_sqrt ( GMP $a )
func gmpSqrt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_sqrt(): Argument #1 ($num) must be greater than or equal to 0")
	}

	r := &big.Int{}
	r.Sqrt(i)

	return returnInt(ctx, r)
}

// > func array gmp_sqrtrem ( GMP $a )
func gmpSqrtrem(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_sqrtrem(): Argument #1 ($num) must be greater than or equal to 0")
	}

	root := new(big.Int).Sqrt(i)
	rem := new(big.Int).Sub(i, new(big.Int).Mul(root, root))

	rootVal, err := returnInt(ctx, root)
	if err != nil {
		return nil, err
	}
	remVal, err := returnInt(ctx, rem)
	if err != nil {
		return nil, err
	}

	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, nil, rootVal)
	arr.OffsetSet(ctx, nil, remVal)

	return arr.ZVal(), nil
}

// > func GMP gmp_root ( GMP $num , int $nth )
func gmpRoot(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var nth phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &nth)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if nth <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_root(): Argument #2 ($nth) must be positive")
	}

	if i.Sign() < 0 && nth%2 == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_root(): Argument #1 ($num) must be positive if argument #2 ($nth) is even")
	}

	negative := i.Sign() < 0
	absI := new(big.Int).Abs(i)

	root := intNthRoot(absI, int(nth))
	if negative {
		root.Neg(root)
	}

	return returnInt(ctx, root)
}

// > func array gmp_rootrem ( GMP $num , int $nth )
func gmpRootrem(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var nth phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &nth)
	if err != nil {
		return nil, err
	}

	i, err := readInt(ctx, a)
	if err != nil {
		return nil, err
	}

	if nth <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_rootrem(): Argument #2 ($nth) must be positive")
	}

	if i.Sign() < 0 && nth%2 == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_rootrem(): Argument #1 ($num) must be positive if argument #2 ($nth) is even")
	}

	negative := i.Sign() < 0
	absI := new(big.Int).Abs(i)

	root := intNthRoot(absI, int(nth))
	if negative {
		root.Neg(root)
	}

	// remainder = num - root^nth
	rootPow := new(big.Int).Exp(root, big.NewInt(int64(nth)), nil)
	rem := new(big.Int).Sub(i, rootPow)

	rootVal, err := returnInt(ctx, root)
	if err != nil {
		return nil, err
	}
	remVal, err := returnInt(ctx, rem)
	if err != nil {
		return nil, err
	}

	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, nil, rootVal)
	arr.OffsetSet(ctx, nil, remVal)

	return arr.ZVal(), nil
}

// intNthRoot computes the integer nth root of x (floor(x^(1/n))).
// Uses Newton's method.
func intNthRoot(x *big.Int, n int) *big.Int {
	if x.Sign() == 0 {
		return big.NewInt(0)
	}
	if n == 1 {
		return new(big.Int).Set(x)
	}
	if n == 2 {
		return new(big.Int).Sqrt(x)
	}

	// Initial guess using bit length
	bitLen := x.BitLen()
	guessBits := (bitLen + n - 1) / n
	if guessBits < 1 {
		guessBits = 1
	}

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

	// Verify: check guess and guess+1, return the floor root
	result := new(big.Int).Exp(guess, nBig, nil)
	if result.Cmp(x) > 0 {
		// guess is too large, try guess-1
		guess.Sub(guess, big.NewInt(1))
	} else {
		// Check if guess+1 is still a valid floor root
		guessPlusOne := new(big.Int).Add(guess, big.NewInt(1))
		resultP1 := new(big.Int).Exp(guessPlusOne, nBig, nil)
		if resultP1.Cmp(x) <= 0 {
			guess = guessPlusOne
		}
	}

	return guess
}

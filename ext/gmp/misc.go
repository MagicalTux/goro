package gmp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	mathrand "math/rand"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/goark/mt/mt19937"
)

// gmpRandKey identifies the per-request seeded MT19937 source used by
// gmp_random_bits / gmp_random_range. The value is a *mathrand.Rand (nil
// means "no seed set yet, use crypto/rand").
var gmpRandKey = phpv.NewStateKey("gmp.rand")

func gmpRand(ctx phpv.Context) *mathrand.Rand {
	if v := ctx.Global().State(gmpRandKey); v != nil {
		return v.(*mathrand.Rand)
	}
	return nil
}

// > func bool gmp_perfect_power ( GMP $num )
func gmpPerfectPower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal

	_, err := core.Expand(ctx, args, &a)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, a, "gmp_perfect_power", 1, "num")
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

// gmpExportBytes converts a big.Int to a byte slice with the given word size and ordering options.
// opts: GMP_BIG_ENDIAN=0x02, GMP_LITTLE_ENDIAN=0x04, GMP_MSW_FIRST=0x08, GMP_LSW_FIRST=0x10
// Default: GMP_MSW_FIRST | GMP_BIG_ENDIAN
func gmpExportBytes(num *big.Int, wordSize int, opts int) []byte {
	// For zero, return empty string (PHP behavior)
	if num.Sign() == 0 {
		return []byte{}
	}

	// Get the big-endian bytes
	raw := num.Bytes()

	// Pad to multiple of wordSize
	if wordSize > 1 && len(raw)%wordSize != 0 {
		padding := wordSize - (len(raw) % wordSize)
		padded := make([]byte, padding+len(raw))
		copy(padded[padding:], raw)
		raw = padded
	}

	// raw is now MSW-first, big-endian within each word
	// Apply word ordering and byte ordering

	// Determine word order: LSW_FIRST=0x10, default=MSW_FIRST=0x08
	lswFirst := (opts & 0x10) != 0
	// Determine byte order: LITTLE_ENDIAN=0x04, default=BIG_ENDIAN=0x02
	littleEndian := (opts & 0x04) != 0

	numWords := len(raw) / wordSize
	result := make([]byte, len(raw))

	for w := 0; w < numWords; w++ {
		// Source word index (from raw, which is MSW-first)
		srcWord := w
		// Dest word index
		dstWord := w
		if lswFirst {
			dstWord = numWords - 1 - w
		}

		// Copy word bytes from src to dst
		srcOff := srcWord * wordSize
		dstOff := dstWord * wordSize

		if littleEndian {
			// Reverse bytes within the word
			for j := 0; j < wordSize; j++ {
				result[dstOff+j] = raw[srcOff+(wordSize-1-j)]
			}
		} else {
			copy(result[dstOff:dstOff+wordSize], raw[srcOff:srcOff+wordSize])
		}
	}

	return result
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

	i, err := readIntArg(ctx, a, "gmp_export", 1, "num")
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

	// Check that word_size is not too large (would overflow allocation)
	const maxExportWordSize = 1 << 24 // 16MB max word size
	if int64(ws) > maxExportWordSize {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"gmp_export(): Argument #2 ($word_size) is too large for argument #1 ($num)")
	}

	// Check that word_size is not larger than the number's byte representation
	// (PHP throws ValueError: word_size larger than number)
	if i.Sign() != 0 {
		numBytes := int64(len(i.Bytes()))
		wsInt := int64(ws)
		// Calculate required bytes (padded to word_size boundary)
		paddedBytes := wsInt
		if numBytes > wsInt {
			paddedBytes = ((numBytes + wsInt - 1) / wsInt) * wsInt
		}
		if paddedBytes > maxExportWordSize {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"gmp_export(): Argument #2 ($word_size) is too large for argument #1 ($num)")
		}
	} else if int64(ws) > 1 {
		// For zero, word_size > 1 would need padding but PHP returns empty string
	}

	// Default options: GMP_MSW_FIRST | GMP_BIG_ENDIAN = 0x08 | 0x02 = 0x0a
	opts := 0x0a
	if options != nil {
		opts = int(*options)
		// GMP_BIG_ENDIAN = 0x02, GMP_LITTLE_ENDIAN = 0x04, GMP_MSW_FIRST = 0x08, GMP_LSW_FIRST = 0x10
		wordOrder := opts & 0x18 // MSW_FIRST | LSW_FIRST bits
		byteOrder := opts & 0x06 // BIG_ENDIAN | LITTLE_ENDIAN bits
		if wordOrder == 0x18 { // Both MSW and LSW set
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_export(): Argument #3 ($flags) cannot use multiple word order options")
		}
		if byteOrder == 0x06 { // Both BIG and LITTLE set
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_export(): Argument #3 ($flags) cannot use multiple endian options")
		}
	}

	b := gmpExportBytes(i, int(ws), opts)
	return phpv.ZString(b).ZVal(), nil
}

// > func GMP gmp_import ( string $data [, int $word_size = 1 [, int $options = GMP_MSW_FIRST | GMP_BIG_ENDIAN ]] )
func gmpImport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var wordSize *phpv.ZInt
	var options *phpv.ZInt

	_, err := core.Expand(ctx, args, &data, &wordSize, &options)
	if err != nil {
		return nil, err
	}

	ws := phpv.ZInt(1)
	if wordSize != nil {
		ws = *wordSize
	}

	if ws <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_import(): Argument #2 ($word_size) must be greater than or equal to 1")
	}

	b := []byte(data)
	if len(b) == 0 {
		return returnInt(ctx, big.NewInt(0))
	}

	if int(ws) > 1 && len(b)%int(ws) != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_import(): Argument #1 ($data) must be a multiple of argument #2 ($word_size)")
	}

	// Default options: GMP_MSW_FIRST | GMP_BIG_ENDIAN = 0x0a
	opts := 0x0a
	if options != nil {
		opt := int(*options)
		wordOrder := opt & 0x18
		byteOrder := opt & 0x06
		if wordOrder == 0x18 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_import(): Argument #3 ($flags) cannot use multiple word order options")
		}
		if byteOrder == 0x06 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_import(): Argument #3 ($flags) cannot use multiple endian options")
		}
		opts = opt
	}

	// Convert input bytes to canonical big-endian MSW-first format
	wordSize2 := int(ws)
	lswFirst := (opts & 0x10) != 0
	littleEndian := (opts & 0x04) != 0

	numWords := len(b) / wordSize2
	canonical := make([]byte, len(b))

	for w := 0; w < numWords; w++ {
		// Source word in input
		srcWord := w
		// In canonical form (MSW-first), dest word position
		dstWord := w
		if lswFirst {
			// Input has LSW first, so word 0 is the least significant
			// We need to reverse the word order for canonical MSW-first
			srcWord = numWords - 1 - w
		}

		srcOff := srcWord * wordSize2
		dstOff := dstWord * wordSize2

		if littleEndian {
			// Bytes within word are little-endian (LSB first), reverse them
			for j := 0; j < wordSize2; j++ {
				canonical[dstOff+j] = b[srcOff+(wordSize2-1-j)]
			}
		} else {
			copy(canonical[dstOff:dstOff+wordSize2], b[srcOff:srcOff+wordSize2])
		}
	}

	i := new(big.Int).SetBytes(canonical)
	return returnInt(ctx, i)
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

	var r *big.Int
	if src := gmpRand(ctx); src != nil {
		// Seeded deterministic source set by gmp_random_seed().
		r = new(big.Int)
		r.Rand(src, max)
	} else {
		var err2 error
		r, err2 = rand.Int(rand.Reader, max)
		if err2 != nil {
			return nil, err2
		}
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

	ia, err := readIntArg(ctx, a, "gmp_random_range", 1, "min")
	if err != nil {
		return nil, err
	}
	ib, err := readIntArg(ctx, b, "gmp_random_range", 2, "max")
	if err != nil {
		return nil, err
	}

	if ia.Cmp(ib) > 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"gmp_random_range(): Argument #1 ($min) must be less than argument #2 ($maximum)")
	}

	// range = max - min + 1
	rangeVal := new(big.Int).Sub(ib, ia)
	rangeVal.Add(rangeVal, big.NewInt(1))

	var r *big.Int
	if src := gmpRand(ctx); src != nil {
		r = new(big.Int)
		r.Rand(src, rangeVal)
	} else {
		var err2 error
		r, err2 = rand.Int(rand.Reader, rangeVal)
		if err2 != nil {
			return nil, err2
		}
	}

	r.Add(r, ia)

	return returnInt(ctx, r)
}

// > func void gmp_random_seed ( GMP $seed )
func gmpRandomSeed(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var seed *phpv.ZVal

	_, err := core.Expand(ctx, args, &seed)
	if err != nil {
		return nil, err
	}

	iseed, err := readIntArg(ctx, seed, "gmp_random_seed", 1, "seed")
	if err != nil {
		return nil, err
	}

	// Derive a 64-bit seed from the GMP bignum. GMP seeds are arbitrary-
	// precision but MT19937 takes a single int64, so we fold the big end
	// of the value down and preserve the sign.
	seedBytes := iseed.Bytes()
	var seedVal int64
	if len(seedBytes) >= 8 {
		seedVal = int64(binary.BigEndian.Uint64(seedBytes[:8]))
	} else {
		padded := make([]byte, 8)
		copy(padded[8-len(seedBytes):], seedBytes)
		seedVal = int64(binary.BigEndian.Uint64(padded))
	}
	if iseed.Sign() < 0 {
		seedVal = -seedVal
	}

	// Seed a dedicated MT19937 instance for this request's gmp_random_*
	// calls. Independent of Mt (mt_rand) so the two don't disturb each
	// other, and scoped to the Global so it can't leak across requests.
	//
	// Note: still doesn't byte-match PHP's libgmp, because libgmp
	// initializes its own MT state differently from the reference
	// algorithm. That's tracked in php_test.go's skip list.
	ctx.Global().SetState(gmpRandKey, mathrand.New(mt19937.New(seedVal)))

	return nil, nil
}

// > func int gmp_scan0 ( GMP $num , int $start )
func gmpScan0(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var a *phpv.ZVal
	var start phpv.ZInt

	_, err := core.Expand(ctx, args, &a, &start)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, a, "gmp_scan0", 1, "num1")
	if err != nil {
		return nil, err
	}

	if start < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_scan0(): Argument #2 ($start) must be between 0 and %d * %d", math.MaxInt64, 8))
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

	i, err := readIntArg(ctx, a, "gmp_scan1", 1, "num1")
	if err != nil {
		return nil, err
	}

	if start < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("gmp_scan1(): Argument #2 ($start) must be between 0 and %d * %d", math.MaxInt64, 8))
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

// > func GMP gmp_binomial ( GMP|int $n , int $k )
func gmpBinomial(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var n *phpv.ZVal
	var k phpv.ZInt

	_, err := core.Expand(ctx, args, &n, &k)
	if err != nil {
		return nil, err
	}

	in, err := readIntArg(ctx, n, "gmp_binomial", 1, "n")
	if err != nil {
		return nil, err
	}

	if k < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_binomial(): Argument #2 ($k) must be greater than or equal to 0")
	}

	var r *big.Int
	if in.Sign() < 0 {
		// For negative n, use the formula: C(-n, k) = (-1)^k * C(n+k-1, k)
		// where the inner binomial uses positive values.
		// Equivalently: C(n, k) where n < 0 and k >= 0
		// = (-1)^k * C(-n+k-1, k)
		absN := new(big.Int).Neg(in) // -n (positive)
		// -n + k - 1
		inner := new(big.Int).Add(absN, big.NewInt(int64(k)-1))
		r = new(big.Int).Binomial(inner.Int64(), int64(k))
		if k%2 != 0 {
			r.Neg(r)
		}
	} else {
		r = new(big.Int).Binomial(in.Int64(), int64(k))
	}
	return returnInt(ctx, r)
}

package gmp

import (
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// phpGMPDigits is the character table PHP GMP uses for base conversion.
// Unlike Go's big.Int.Text() which uses 0-9, a-z, A-Z,
// PHP GMP uses 0-9, A-Z, a-z (uppercase before lowercase for digits 10-61).
const phpGMPDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// bigIntText converts a big.Int to a string using PHP GMP's character table.
// For bases 2-36, this matches Go's output (but we still use our table for consistency).
// For bases 37-62, PHP uses uppercase letters (A-Z) for digits 10-35 and lowercase (a-z) for 36-61.
// Negative base means uppercase output (only applies to bases 2-36).
func bigIntToStringGMP(n *big.Int, base int, uppercase bool) string {
	if base < 2 || base > 62 {
		return ""
	}

	// For bases <= 36, Go's Text() output matches what PHP expects (for positive bases).
	// For negative base (uppercase), we need all uppercase.
	if base <= 36 {
		result := n.Text(base)
		if uppercase {
			result = strings.ToUpper(result)
		}
		return result
	}

	// For bases 37-62, implement custom conversion using PHP's digit table
	zero := new(big.Int)
	if n.Cmp(zero) == 0 {
		return "0"
	}

	negative := n.Sign() < 0
	abs := new(big.Int).Abs(n)
	bigBase := big.NewInt(int64(base))

	var digits []byte
	mod := new(big.Int)
	for abs.Cmp(zero) > 0 {
		abs.DivMod(abs, bigBase, mod)
		digits = append(digits, phpGMPDigits[mod.Int64()])
	}

	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}

	result := string(digits)
	if negative {
		result = "-" + result
	}
	return result
}

// > func string gmp_strval ( GMP $gmpnumber [, int $base = 10 ] )
func gmpStrval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal
	var base *phpv.ZInt

	_, err := core.Expand(ctx, args, &gmpnum, &base)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, gmpnum, "gmp_strval", 1, "num")
	if err != nil {
		return nil, err
	}

	if base == nil {
		base = new(phpv.ZInt)
		*base = 10
	}

	b := int(*base)
	// Validate base: must be 2-62 or -36 to -2
	if (b < 2 || b > 62) && (b < -36 || b > -2) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_strval(): Argument #2 ($base) must be between 2 and 62, or -2 and -36")
	}

	// Negative base means uppercase letters (only 2-36 range)
	uppercase := b < 0
	if b < 0 {
		b = -b
	}

	result := bigIntToStringGMP(i, b, uppercase)

	return phpv.ZString(result).ZVal(), nil
}

// > func int gmp_intval ( GMP $gmpnumber )
func gmpIntval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var gmpnum *phpv.ZVal

	_, err := core.Expand(ctx, args, &gmpnum)
	if err != nil {
		return nil, err
	}

	i, err := readIntArg(ctx, gmpnum, "gmp_intval", 1, "num")
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(i.Int64()).ZVal(), nil
}

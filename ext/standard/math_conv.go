package standard

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string decbin ( float $number )
func mathDecBin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%b", uint64(num))), nil
}

// > func string dechex ( float $number )
func mathDecHex(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%x", uint64(num))), nil
}

// > func string decoct ( float $number )
func mathDecOct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZInt
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZStr(fmt.Sprintf("%o", uint64(num))), nil
}

// isValidForBase checks if a character is valid in the given base
func isValidForBase(c rune, base int) bool {
	if c >= '0' && c <= '9' {
		return int(c-'0') < base
	}
	if c >= 'a' && c <= 'z' {
		return int(c-'a'+10) < base
	}
	if c >= 'A' && c <= 'Z' {
		return int(c-'A'+10) < base
	}
	return false
}

// filterAndWarnInvalid filters a string keeping only valid chars for the given base,
// and emits a deprecation warning if invalid characters were found.
func filterAndWarnInvalid(ctx phpv.Context, s string, base int) (string, error) {
	var buf bytes.Buffer
	hasInvalid := false

	for _, c := range s {
		if isValidForBase(c, base) {
			buf.WriteRune(c)
		} else {
			hasInvalid = true
		}
	}

	if hasInvalid {
		if err := ctx.Deprecated("Invalid characters passed for attempted conversion, these have been ignored", logopt.NoFuncName(true)); err != nil {
			return buf.String(), err
		}
	}

	return buf.String(), nil
}

// > func number bindec ( string $binary_string )
func mathBinDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	s, err := filterAndWarnInvalid(ctx, string(num), 2)
	if err != nil {
		return phpv.ZInt(0).ZVal(), err
	}

	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	return ParseInt(s, 2, 64)
}

// > func number octdec ( string $oct_string )
func mathOctDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	s, err := filterAndWarnInvalid(ctx, string(num), 8)
	if err != nil {
		return phpv.ZInt(0).ZVal(), err
	}

	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	return ParseInt(s, 8, 64)
}

// > func number hexdec ( string $hex_string )
func mathHexDec(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	_, err := core.Expand(ctx, args, &num)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if len(num) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	s, err := filterAndWarnInvalid(ctx, string(num), 16)
	if err != nil {
		return phpv.ZInt(0).ZVal(), err
	}

	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	return ParseInt(s, 16, 64)
}

// stripBasePrefix strips recognized base prefixes (0x, 0b, 0o) for the given base,
// and also strips leading/trailing whitespace. This implements PHP 8.4+ base_convert improvements.
func stripBasePrefix(s string, base int) string {
	// Strip leading/trailing whitespace
	s = strings.TrimSpace(s)

	if len(s) < 2 {
		return s
	}

	// Strip recognized prefix for the given base
	prefix := strings.ToLower(s[:2])
	switch {
	case base == 16 && prefix == "0x":
		s = s[2:]
	case base == 2 && prefix == "0b":
		s = s[2:]
	case base == 8 && prefix == "0o":
		s = s[2:]
	}

	// Strip leading zeros
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}

	return s
}

// > func string base_convert ( string $number , int $frombase , int $tobase )
func mathBaseConvert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZString
	var fromBase, toBase phpv.ZInt
	_, err := core.Expand(ctx, args, &num, &fromBase, &toBase)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if fromBase < 2 || fromBase > 36 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "base_convert(): Argument #2 ($from_base) must be between 2 and 36 (inclusive)")
	}
	if toBase < 2 || toBase > 36 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "base_convert(): Argument #3 ($to_base) must be between 2 and 36 (inclusive)")
	}

	// Strip whitespace and base prefix
	s := stripBasePrefix(string(num), int(fromBase))

	// Lowercase for consistent processing
	s = strings.ToLower(s)

	// Filter invalid characters and emit deprecation warning
	filtered, err := filterAndWarnInvalid(ctx, s, int(fromBase))
	if err != nil {
		// If error from deprecation warning, still return the result
		if filtered == "" {
			return phpv.ZString("0").ZVal(), err
		}
	}

	if filtered == "" {
		return phpv.ZString("0").ZVal(), nil
	}

	// Use big.Int for arbitrary precision base conversion
	n := new(big.Int)
	_, ok := n.SetString(filtered, int(fromBase))
	if !ok {
		return phpv.ZString("0").ZVal(), nil
	}

	result := strings.ToLower(n.Text(int(toBase)))

	return phpv.ZStr(result), nil
}

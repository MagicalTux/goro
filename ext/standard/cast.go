package standard

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool boolval ( mixed $var )
func fncBoolval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtBool)
}

// > func float doubleval ( mixed $var )
func fncDoubleval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncFloatval(ctx, args)
}

// > func float floatval ( mixed $var )
func fncFloatval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtFloat)
}

// > func int intval ( mixed $var [, int $base = 10 ] )
func fncIntval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	var base *phpv.ZInt
	_, err := core.Expand(ctx, args, &v, &base)
	if err != nil {
		return nil, err
	}

	b := 10
	if base != nil {
		b = int(*base)
	}

	// If no base specified (or base=10) and value is not a string, use normal int conversion
	if base == nil && v.GetType() != phpv.ZtString {
		return v.As(ctx, phpv.ZtInt)
	}

	// For non-string values with explicit base, convert to string first
	s := strings.TrimSpace(v.String())
	if s == "" {
		return phpv.ZInt(0).ZVal(), nil
	}

	// Base 0 means auto-detect
	if b == 0 {
		return phpv.ZInt(phpIntvalBase0(s)).ZVal(), nil
	}

	if b < 2 || b > 36 {
		return phpv.ZInt(0).ZVal(), nil
	}

	// Handle sign
	negative := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			negative = true
		}
		s = s[1:]
	}

	// Strip prefix for specific bases
	if b == 16 && len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	} else if b == 2 && len(s) >= 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B') {
		s = s[2:]
	} else if b == 8 && len(s) >= 2 && s[0] == '0' && (s[1] == 'o' || s[1] == 'O') {
		s = s[2:]
	}

	// Parse digit by digit, stopping at first invalid digit
	var result int64
	for _, c := range s {
		var digit int64
		if c >= '0' && c <= '9' {
			digit = int64(c - '0')
		} else if c >= 'a' && c <= 'z' {
			digit = int64(c-'a') + 10
		} else if c >= 'A' && c <= 'Z' {
			digit = int64(c-'A') + 10
		} else {
			break // stop at first non-digit character
		}
		if digit >= int64(b) {
			break // stop at digit out of range for this base
		}
		result = result*int64(b) + digit
	}

	if negative {
		result = -result
	}
	return phpv.ZInt(result).ZVal(), nil
}

// phpIntvalBase0 handles base-0 auto-detection for intval()
func phpIntvalBase0(s string) int64 {
	negative := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			negative = true
		}
		s = s[1:]
	}

	var result int64
	base := 10

	if len(s) >= 2 && s[0] == '0' {
		switch {
		case s[1] == 'x' || s[1] == 'X':
			base = 16
			s = s[2:]
		case s[1] == 'b' || s[1] == 'B':
			base = 2
			s = s[2:]
		case s[1] == 'o' || s[1] == 'O':
			base = 8
			s = s[2:]
		default:
			base = 8
			s = s[1:]
		}
	}

	for _, c := range s {
		var digit int64
		if c >= '0' && c <= '9' {
			digit = int64(c - '0')
		} else if c >= 'a' && c <= 'z' {
			digit = int64(c-'a') + 10
		} else if c >= 'A' && c <= 'Z' {
			digit = int64(c-'A') + 10
		} else {
			break
		}
		if digit >= int64(base) {
			break
		}
		result = result*int64(base) + digit
	}

	if negative {
		result = -result
	}
	return result
}

// > func string strval ( mixed $var )
func fncStrval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	return v.As(ctx, phpv.ZtString)
}

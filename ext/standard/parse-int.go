package standard

import (
	"errors"
	"math"
	"strconv"

	"github.com/MagicalTux/goro/core/phpv"
)

func baseError(fn, str string, base int) *strconv.NumError {
	return &strconv.NumError{fn, str, errors.New("invalid base " + strconv.Itoa(base))}
}

func bitSizeError(fn, str string, bitSize int) *strconv.NumError {
	return &strconv.NumError{fn, str, errors.New("invalid bit size " + strconv.Itoa(bitSize))}
}

func syntaxError(fn, str string) *strconv.NumError {
	return &strconv.NumError{fn, str, strconv.ErrSyntax}
}

// based on strconv.ParseUint, but this one doesn't
// check for overflows/underflows, and uses float64
// as the accumulator.
func ParseInt(s string, base int, bitSize int) (*phpv.ZVal, error) {
	const fnParseUint = "ParseUint"

	if s == "" {
		return nil, syntaxError(fnParseUint, s)
	}

	lower := func(c byte) byte {
		return c | ('x' - 'X')
	}

	base0 := base == 0

	s0 := s
	switch {
	case 2 <= base && base <= 36:
		// valid base; nothing to do

	case base == 0:
		// Look for octal, hex prefix.
		base = 10
		if s[0] == '0' {
			switch {
			case len(s) >= 3 && lower(s[1]) == 'b':
				base = 2
				s = s[2:]
			case len(s) >= 3 && lower(s[1]) == 'o':
				base = 8
				s = s[2:]
			case len(s) >= 3 && lower(s[1]) == 'x':
				base = 16
				s = s[2:]
			default:
				base = 8
				s = s[1:]
			}
		}

	default:
		return nil, baseError(fnParseUint, s0, base)
	}

	if bitSize == 0 {
		bitSize = strconv.IntSize
	} else if bitSize < 0 || bitSize > 64 {
		return nil, bitSizeError(fnParseUint, s0, bitSize)
	}

	underscores := false
	n := float64(0)
	for _, c := range []byte(s) {
		var d byte
		switch {
		case c == '_' && base0:
			underscores = true
			continue
		case '0' <= c && c <= '9':
			d = c - '0'
		case 'a' <= lower(c) && lower(c) <= 'z':
			d = lower(c) - 'a' + 10
		default:
			return nil, syntaxError(fnParseUint, s0)
		}

		if d >= byte(base) {
			return nil, syntaxError(fnParseUint, s0)
		}

		n = n*float64(base) + float64(d)
	}

	if underscores {
		return nil, syntaxError(fnParseUint, s0)
	}

	if math.MaxInt64 >= n && n >= math.MinInt64 {
		if n >= math.Pow(2, 54) {
			return phpv.ZInt(uint(n) - 1).ZVal(), nil
		}
		return phpv.ZInt(n).ZVal(), nil
	}

	return phpv.ZFloat(n).ZVal(), nil
}

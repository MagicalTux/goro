package gmp

import (
	"math/big"
	"strings"
	"unicode"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// stripWhitespace removes all whitespace characters from a string.
// PHP GMP allows whitespace within numeric strings when a base is given.
func stripWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// > func GMP gmp_init ( mixed $number [, int $base = 0 ] )
func gmpInit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num *phpv.ZVal
	var base *phpv.ZInt

	_, err := core.Expand(ctx, args, &num, &base)
	if err != nil {
		return nil, err
	}

	// Check if num is a GMP object - PHP disallows this
	if num.GetType() == phpv.ZtObject {
		if obj, ok := num.Value().(*phpobj.ZObject); ok && obj.Class == GMP {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "gmp_init(): Argument #1 ($num) must be of type string|int, GMP given")
		}
	}

	// Validate base
	if base != nil {
		b := int(*base)
		if b != 0 && (b < 2 || b > 62) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #2 ($base) must be 0 or between 2 and 62")
		}
	}

	var i *big.Int

	switch num.GetType() {
	case phpv.ZtNull, phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat:
		num, err = num.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		i = big.NewInt(int64(num.Value().(phpv.ZInt)))
	default:
		num, err = num.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		s := string(num.AsString(ctx))
		s = strings.TrimSpace(s)
		i = &big.Int{}
		b := 0
		if base != nil {
			b = int(*base)
		}
		if s == "" {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #1 ($num) is not an integer string")
		}
		// PHP does not accept leading '+' sign
		if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-+") {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #1 ($num) is not an integer string")
		}
		// When an explicit base is given, allow matching prefix (0b for base 2, 0x for base 16, 0o/0 for base 8)
		// Go's big.Int.SetString only recognizes these prefixes with base 0
		parseStr := s
		parseBase := b
		if b != 0 {
			neg := strings.HasPrefix(s, "-")
			body := s
			if neg {
				body = s[1:]
			}
			// PHP GMP allows whitespace throughout the string - strip internal whitespace
			body = stripWhitespace(body)
			switch b {
			case 2:
				if strings.HasPrefix(strings.ToLower(body), "0b") {
					body = body[2:]
					parseBase = 2
				}
			case 16:
				if strings.HasPrefix(strings.ToLower(body), "0x") {
					body = body[2:]
					parseBase = 16
				}
			case 8:
				if strings.HasPrefix(strings.ToLower(body), "0o") {
					body = body[2:]
					parseBase = 8
				} else if strings.HasPrefix(body, "0") && len(body) > 1 {
					body = body[1:]
					parseBase = 8
				}
			}
			if neg {
				parseStr = "-" + body
			} else {
				parseStr = body
			}
		}
		_, ok := i.SetString(parseStr, parseBase)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "gmp_init(): Argument #1 ($num) is not an integer string")
		}
	}

	return returnInt(ctx, i)
}

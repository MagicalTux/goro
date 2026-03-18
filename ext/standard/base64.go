package standard

import (
	"encoding/base64"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string base64_encode ( string $data )
func fncBase64Encode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	err = ctx.MemAlloc(ctx, uint64(base64.StdEncoding.EncodedLen(len(s))))
	if err != nil {
		return nil, err
	}

	r := base64.StdEncoding.EncodeToString([]byte(s))
	return phpv.ZString(r).ZVal(), nil
}

// phpBase64Valid checks if a byte is a valid base64 character
func phpBase64Valid(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/'
}

// phpBase64Clean strips all non-base64 characters for non-strict mode (PHP behavior)
func phpBase64Clean(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if phpBase64Valid(c) || c == '=' {
			buf.WriteByte(c)
		}
	}
	return buf.String()
}

// > func string base64_decode ( string $data [, bool $strict = FALSE ] )
func fncBase64Decode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var strict *phpv.ZBool
	_, err := core.Expand(ctx, args, &s, &strict)

	err = ctx.MemAlloc(ctx, uint64(base64.StdEncoding.DecodedLen(len(s))))
	if err != nil {
		return nil, err
	}

	if strict != nil && *strict {
		// Strip whitespace for strict mode too (PHP does this)
		cleaned := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				return -1
			}
			return r
		}, string(s))

		// Validate: only base64 chars and padding allowed
		padStarted := false
		for i := 0; i < len(cleaned); i++ {
			c := cleaned[i]
			if c == '=' {
				padStarted = true
			} else if padStarted {
				return phpv.ZFalse.ZVal(), nil
			} else if !phpBase64Valid(c) {
				return phpv.ZFalse.ZVal(), nil
			}
		}

		// Add padding if needed
		if mod := len(cleaned) % 4; mod != 0 {
			cleaned += strings.Repeat("=", 4-mod)
		}

		r, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZString(r).ZVal(), nil
	}

	// non strict mode: PHP silently ignores any non-base64 characters
	cleaned := phpBase64Clean(string(s))
	cleaned = strings.TrimRight(cleaned, "=")

	if len(cleaned) == 0 {
		return phpv.ZString("").ZVal(), nil
	}

	r, err := base64.RawStdEncoding.DecodeString(cleaned)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(r).ZVal(), nil
}

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

func phpBase64IsWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// > func string|false base64_decode ( string $data [, bool $strict = FALSE ] )
func fncBase64Decode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var strict *phpv.ZBool
	_, err := core.Expand(ctx, args, &s, &strict)

	err = ctx.MemAlloc(ctx, uint64(base64.StdEncoding.DecodedLen(len(s))))
	if err != nil {
		return nil, err
	}

	if strict != nil && *strict {
		// Strict: strip whitespace, validate chars and padding
		var cleaned strings.Builder
		for i := 0; i < len(s); i++ {
			if !phpBase64IsWhitespace(s[i]) {
				cleaned.WriteByte(s[i])
			}
		}
		data := cleaned.String()
		if data == "" {
			return phpv.ZString("").ZVal(), nil
		}
		// Find data vs padding boundary
		dataEnd := len(data)
		for dataEnd > 0 && data[dataEnd-1] == '=' {
			dataEnd--
		}
		dataChars := data[:dataEnd]
		padChars := data[dataEnd:]
		for i := 0; i < len(dataChars); i++ {
			if !phpBase64Valid(dataChars[i]) {
				return phpv.ZFalse.ZVal(), nil
			}
		}
		for i := 0; i < len(padChars); i++ {
			if padChars[i] != '=' {
				return phpv.ZFalse.ZVal(), nil
			}
		}
		mod := len(dataChars) % 4
		if mod == 1 {
			return phpv.ZFalse.ZVal(), nil
		}
		expectedPad := 0
		if mod == 2 {
			expectedPad = 2
		} else if mod == 3 {
			expectedPad = 1
		}
		if len(padChars) != expectedPad && len(padChars) != 0 {
			return phpv.ZFalse.ZVal(), nil
		}
		padded := dataChars
		if mod != 0 {
			padded += strings.Repeat("=", 4-mod)
		}
		r, decErr := base64.StdEncoding.DecodeString(padded)
		if decErr != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZString(r).ZVal(), nil
	}

	// Non-strict: strip ALL non-base64 chars (including =)
	var cleaned strings.Builder
	cleaned.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if phpBase64Valid(s[i]) {
			cleaned.WriteByte(s[i])
		}
	}
	data := cleaned.String()
	if len(data) == 0 {
		return phpv.ZString("").ZVal(), nil
	}
	// Truncate trailing incomplete group (mod 4 == 1 means lone char)
	if mod := len(data) % 4; mod == 1 {
		data = data[:len(data)-1]
		if len(data) == 0 {
			return phpv.ZString("").ZVal(), nil
		}
	}
	if mod2 := len(data) % 4; mod2 != 0 {
		data += strings.Repeat("=", 4-mod2)
	}
	r, decErr := base64.StdEncoding.DecodeString(data)
	if decErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(r).ZVal(), nil
}

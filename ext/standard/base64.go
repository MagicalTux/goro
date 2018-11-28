package standard

import (
	"encoding/base64"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string base64_encode ( string $data )
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

//> func string base64_decode ( string $data [, bool $strict = FALSE ] )
func fncBase64Decode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var strict *phpv.ZBool
	_, err := core.Expand(ctx, args, &s, &strict)

	err = ctx.MemAlloc(ctx, uint64(base64.StdEncoding.DecodedLen(len(s))))
	if err != nil {
		return nil, err
	}

	if strict != nil && *strict {
		// strict mode (TODO not sure it has the same meaning in Go, to be tested)
		r, err := base64.StdEncoding.Strict().DecodeString(string(s))
		if err != nil {
			return nil, err
		}
		return phpv.ZString(r).ZVal(), nil
	}

	// non strict mode
	// note: php base64 will accept missing trailing, which PHP has trouble with, so we'll just trim =
	r, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(string(s), "\r\n ="))
	if err != nil {
		return nil, err
	}
	return phpv.ZString(r).ZVal(), nil
}

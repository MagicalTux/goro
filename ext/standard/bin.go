package standard

import (
	"encoding/hex"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string bin2hex ( string $str )
func fncBin2hex(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s string

	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(hex.EncodeToString([]byte(s))).ZVal(), nil
}

// > func string hex2bin ( string $str )
func fncHex2Bin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s string

	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	if len(s)%2 != 0 {
		return phpv.ZFalse.ZVal(), ctx.Warn("Hexadecimal input string must have an even length")
	}

	result, err := hex.DecodeString(s)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("Input string must be hexadecimal string")
	}

	return phpv.ZString(result).ZVal(), nil
}

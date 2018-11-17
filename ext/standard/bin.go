package standard

import (
	"encoding/hex"

	"github.com/MagicalTux/gophp/core"
)

//> func string bin2hex ( string $str )
func fncBin2hex(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var s string

	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return core.ZString(hex.EncodeToString([]byte(s))).ZVal(), nil
}

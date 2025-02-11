package standard

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string sscanf ( string $str , string $format [, mixed &$... ] )
func fncSscanf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &str, &fmt)
	if err != nil {
		return nil, err
	}

	r := strings.NewReader(string(str))
	output, err := core.Zscanf(ctx, r, fmt, args[n:]...)
	if err != nil {
		return nil, err
	}

	return output, nil
}

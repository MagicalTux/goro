package standard

import (
	"strconv"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string dechex ( int $number )
func fncDechex(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 16)
	return phpv.ZString(s).ZVal(), nil
}

//> func string decoct ( int $number )
func fncDecoct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 8)
	return phpv.ZString(s).ZVal(), nil
}

//> func string decbin ( int $number )
func fncDecbin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 2)
	return phpv.ZString(s).ZVal(), nil
}

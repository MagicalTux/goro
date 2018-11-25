package standard

import (
	"strconv"

	"github.com/MagicalTux/goro/core"
)

//> func string dechex ( int $number )
func fncDechex(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v core.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 16)
	return core.ZString(s).ZVal(), nil
}

//> func string decoct ( int $number )
func fncDecoct(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v core.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 8)
	return core.ZString(s).ZVal(), nil
}

//> func string decbin ( int $number )
func fncDecbin(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v core.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	s := strconv.FormatInt(int64(v), 2)
	return core.ZString(s).ZVal(), nil
}

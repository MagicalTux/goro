package standard

import (
	"net/url"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string urlencode ( string $str )
func fncUrlencode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(url.QueryEscape(u)).ZVal(), nil
}

// > func string urldecode ( string $str )
func fncUrldecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}
	v, err := url.QueryUnescape(u)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(v).ZVal(), nil
}

// > func string rawurlencode ( string $str )
func fncRawurlencode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(url.PathEscape(u)).ZVal(), nil
}

// > func string rawurldecode ( string $str )
func fncRawurldecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}

	v, err := url.PathUnescape(u)
	if err != nil {
		return nil, err
	}
	return phpv.ZString(v).ZVal(), nil
}

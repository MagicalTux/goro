package standard

import (
	"net/url"

	"github.com/MagicalTux/gophp/core"
)

//> func string urlencode ( string $str )
func fncUrlencode(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}

	return core.ZString(url.QueryEscape(u)).ZVal(), nil
}

//> func string rawurlencode ( string $str )
func fncRawurlencode(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var u string
	_, err := core.Expand(ctx, args, &u)
	if err != nil {
		return nil, err
	}

	return core.ZString(url.PathEscape(u)).ZVal(), nil
}

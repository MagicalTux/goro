package standard

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string urlencode ( string $str )
func fncUrlencode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str []byte
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	// go's url.PathEscape wont't be used since it
	// isn't strictly conformant to RFC 3986
	// and doesn't escape characters like +, $

	var buf bytes.Buffer
	for _, c := range str {
		// all non-alphanumeric characters except -_. will be encoded
		switch c {
		case ' ':
			buf.WriteByte('+')

		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z':
			fallthrough
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
			fallthrough
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			fallthrough
		case '-', '_', '.':
			buf.WriteByte(c)

		default:
			buf.WriteByte('%')
			buf.WriteString(fmt.Sprintf("%02X", c))
		}

	}

	return phpv.ZStr(buf.String()), nil
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
	var str []byte
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	// same as urlencode, except for ' ' and '~'

	var buf bytes.Buffer
	for _, c := range str {
		switch c {
		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z':
			fallthrough
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
			fallthrough
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			fallthrough
		case '-', '_', '.', '~':
			buf.WriteByte(c)

		default:
			buf.WriteByte('%')
			buf.WriteString(fmt.Sprintf("%02X", c))
		}

	}

	return phpv.ZStr(buf.String()), nil
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

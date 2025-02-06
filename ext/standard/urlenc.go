package standard

import (
	"bytes"
	"fmt"
	"strings"

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

		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
			'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
			'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
			'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'-', '_', '.':
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
	u = urlDecode(u, false)
	return phpv.ZString(u).ZVal(), nil
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
		case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
			'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
			'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
			'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'-', '_', '.', '~':
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

	u = urlDecode(u, true)
	return phpv.ZString(u).ZVal(), nil
}

func urlDecode(s string, raw bool) string {
	// url.PathUnescape and url.QueryUnescape aren't used
	// since they error out on the first invalid encoding.
	// PHP's urldecode is lenient, it decodes
	// all valid encoding, other %## are added
	// as is if it's invalid.
	var buf bytes.Buffer
	runes := []rune(s)
	for i := 0; i < len(s); i++ {
		switch c := runes[i]; c {
		default:
			buf.WriteRune(c)
		case '+':
			switch raw {
			case true:
				buf.WriteRune('+')
			case false:
				buf.WriteRune(' ')
			}
		case '%':
			a := core.Idx(runes, i+1)
			b := core.Idx(runes, i+2)
			if !isHex(a) || !isHex(b) {
				buf.WriteRune('%')
				continue
			}
			buf.WriteByte(unhex(byte(a))<<4 | unhex(byte(b)))
			i += 2
		}
	}
	return buf.String()
}

func parseQuery(ctx phpv.Context, query string, array phpv.ZArrayAccess) (err error) {
	getKeyPath := func(key string) (result []string) {
		// getKeyPath("x") == []string{"x"}
		// getKeyPath("x[]") == []string{"x", ""}
		// getKeyPath("x[y]") == []string{"x", "y"}
		// getKeyPath("x[y][]") == []string{"x", "y", ""}
		// getKeyPath("x[") == []string{"x_"}
		// getKeyPath("x][") == []string{"x]_"}
		// getKeyPath("x][[") == []string{"x]_["}
		// getKeyPath("x][[]") == []string{"x]", "["}

		key = strings.TrimSpace(key)
		if len(key) == 0 || key[0] == '[' {
			return
		}

		var i, j int
		i = strings.IndexRune(key, '[')
		if i < 0 {
			result = []string{key}
			goto Return
		}
		j = strings.IndexRune(key[i:], ']')
		if j < 0 {
			result = []string{key}
			goto Return
		}

		result = []string{key[:i], key[i+1 : i+j]}

		key = key[i+j+1:]
		for len(key) > 0 {
			i = strings.IndexRune(key, '[')
			if i < 0 {
				goto Return
			}
			j = strings.IndexRune(key[i:], ']')
			if j < 0 {
				goto Return
			}
			result = append(result, key[i+1:j])
			key = key[j+1:]
		}

	Return:
		tmp := result[0]
		tmp = strings.NewReplacer(".", "_", " ", "_").Replace(tmp)
		i = strings.IndexRune(tmp, '[')
		if i >= 0 {
			// if there's a [ in the key, replace only the first one
			// "key..[abc[[" == "key.._abc[["
			tmp = strings.Replace(tmp[:i+1], "[", "_", 1) + tmp[i+1:]
		}
		result[0] = tmp

		return
	}

	sep := ctx.GetConfig("arg_separator.input", phpv.ZStr("&")).String()

	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, sep)
		if strings.Contains(key, ";") {
			err = fmt.Errorf("invalid semicolon separator in query")
			return
		}
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")

		key = urlDecode(key, false)
		value = urlDecode(value, false)

		container := array
		paths := getKeyPath(key)
		if len(paths) == 0 {
			continue
		}

		// query such as arr[][x]=1234 will create a nested array
		// ["arr" => [ 0 => [x => 1234]]]
		for i := 0; i < len(paths)-1; i++ {
			k := paths[i]
			var arr *phpv.ZArray
			if a, ok, _ := container.OffsetCheck(ctx, phpv.ZStr(k)); ok {
				if a.GetType() != phpv.ZtArray {
					arr = phpv.NewZArray()
				} else {
					arr = a.AsArray(ctx)
				}
			} else {
				arr = phpv.NewZArray()
			}

			if k != "" {
				container.OffsetSet(ctx, phpv.ZStr(k), arr.ZVal())
			} else {
				container.OffsetSet(ctx, nil, arr.ZVal())
			}
			container = arr
		}

		k := paths[len(paths)-1]
		if k != "" {
			container.OffsetSet(ctx, phpv.ZStr(k), phpv.ZStr(value))
		} else {
			container.OffsetSet(ctx, nil, phpv.ZStr(value))
		}

	}

	return err
}

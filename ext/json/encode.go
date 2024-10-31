package json

import (
	"strconv"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

var hex = "0123456789abcdef"

// > func string json_encode ( mixed $value [, int $options = 0 [, int $depth = 512 ]] )
func fncJsonEncode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	var opt, depth *phpv.ZInt
	_, err := core.Expand(ctx, args, &v, &opt, &depth)
	if err != nil {
		return nil, err
	}

	var o JsonEncOpt
	var d = 512
	var r []byte // result

	if opt != nil {
		o = JsonEncOpt(*opt)
	}
	if depth != nil {
		d = int(*depth)
	}

	r, err = appendJsonEncode(ctx, r, v, o, d)

	return phpv.ZString(r).ZVal(), err
}

func appendJsonEncode(ctx phpv.Context, r []byte, v *phpv.ZVal, opt JsonEncOpt, depth int) ([]byte, error) {
	switch v.GetType() {
	case phpv.ZtNull:
		return append(r, []byte("null")...), nil
	case phpv.ZtBool:
		if v.Value().(phpv.ZBool) {
			return append(r, []byte("true")...), nil
		} else {
			return append(r, []byte("false")...), nil
		}
	case phpv.ZtInt:
		s := strconv.FormatInt(int64(v.Value().(phpv.ZInt)), 10)
		return append(r, []byte(s)...), nil
	case phpv.ZtFloat:
		s := strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), 'g', -1, 64)
		return append(r, []byte(s)...), nil
	case phpv.ZtString:
		return appendJsonString(r, string(v.Value().(phpv.ZString)), opt)
	case phpv.ZtArray:
		a := v.Value().(*phpv.ZArray)
		if a.HasStringKeys() {
			// append as object
			return appendJsonObject(ctx, r, a.NewIterator(), opt, depth)
		} else {
			// append as array
			return appendJsonArray(ctx, r, a.NewIterator(), opt, depth)
		}
	case phpv.ZtObject:
		// TODO check for JsonSerializable
		it := v.NewIterator()
		if it == nil {
			return r, ErrUnsupportedType
		}
		return appendJsonObject(ctx, r, it, opt, depth)
	default:
		return r, ErrUnsupportedType
	}
}

func appendJsonArray(ctx phpv.Context, r []byte, it phpv.ZIterator, opt JsonEncOpt, depth int) ([]byte, error) {
	depth = depth - 1
	if depth <= 0 {
		return r, ErrDepth
	}
	r = append(r, '[')
	first := true

	for ; it.Valid(ctx); it.Next(ctx) {
		v, err := it.Current(ctx)
		if err != nil {
			return r, err
		}

		if !first {
			r = append(r, ',')
		}
		first = false

		r, err = appendJsonEncode(ctx, r, v, opt, depth)
		if err != nil {
			return r, err
		}
	}
	r = append(r, ']')
	return r, nil
}

func appendJsonObject(ctx phpv.Context, r []byte, it phpv.ZIterator, opt JsonEncOpt, depth int) ([]byte, error) {
	depth = depth - 1
	if depth <= 0 {
		return r, ErrDepth
	}
	r = append(r, '{')
	first := true

	for ; it.Valid(ctx); it.Next(ctx) {
		k, err := it.Key(ctx)
		if err != nil {
			return r, err
		}
		k, err = k.As(ctx, phpv.ZtString)
		if err != nil {
			return r, err
		}

		v, err := it.Current(ctx)
		if err != nil {
			return r, err
		}

		if !first {
			r = append(r, ',')
		}
		first = false

		r, err = appendJsonString(r, string(k.Value().(phpv.ZString)), opt)
		if err != nil {
			return r, err
		}
		r = append(r, ':')

		r, err = appendJsonEncode(ctx, r, v, opt, depth)
		if err != nil {
			return r, err
		}
	}
	r = append(r, '}')
	return r, nil
}

func appendJsonString(r []byte, s string, opt JsonEncOpt) ([]byte, error) {
	r = append(r, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf { // ASCII
			// check if b is safe
			switch b {
			case '"', '/', '\\':
			default:
				if b >= 0x20 {
					i++
					continue
				}
			}

			if start < i {
				r = append(r, []byte(s[start:i])...)
			}

			// unsafe, check how to escape b
			switch b {
			case '"', '/', '\\':
				// need to prefix a \
				r = append(r, '\\', b)
			case '\n':
				r = append(r, '\\', 'n')
			case '\r':
				r = append(r, '\\', 'r')
			case '\t':
				r = append(r, '\\', 't')
			default:
				// escape as unicode
				r = append(r, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xf])
			}
			i++
			start = i
			continue
		}
		// UTF-8
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				r = append(r, []byte(s[start:i])...)
			}
			if opt&InvalidUtf8Substitute == InvalidUtf8Substitute {
				// substitute character
				r = append(r, []byte(`\ufffd`)...)
			} else if opt&InvalidUtf8Ignore == 0 {
				// return error
				return r, ErrUtf8
			}
			i += size
			start = i
			continue
		}

		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				r = append(r, []byte(s[start:i])...)
			}
			r = append(r, '\\', 'u', '2', '0', '2', hex[c&0xf])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		r = append(r, []byte(s[start:])...)
	}
	r = append(r, '"')
	return r, nil
}

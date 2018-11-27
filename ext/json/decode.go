package json

import (
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
)

//> func mixed json_decode ( string $json [, bool $assoc = FALSE [, int $depth = 512 [, int $options = 0 ]]] )
func fncJsonDecode(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var json core.ZString
	var assoc *core.ZBool
	var depth, opt *core.ZInt

	_, err := core.Expand(ctx, args, &json, &assoc, &depth, &opt)
	if err != nil {
		return nil, err
	}

	var d = 512
	var o JsonDecOpt

	if depth != nil {
		d = int(*depth)
	}
	if opt != nil {
		o = JsonDecOpt(*opt)
	}
	if assoc != nil && *assoc {
		o |= ObjectAsArray
	}

	return jsonDecodeAny(ctx, strings.NewReader(string(json)), d, o)
	// TODO check if reader was fully consumed, return ErrSyntax if not
}

// nextRune returns the next non-space rune
func nextRune(r *strings.Reader) (rune, error) {
	for {
		r, _, err := r.ReadRune()
		if err != nil {
			return r, err
		}
		if !unicode.IsSpace(r) {
			return r, nil
		}
	}
}

func jsonDecodeAny(ctx core.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	b, err := nextRune(r)
	if err != nil {
		return nil, err
	}
	// unread right after reading, we only want to know what we are reading
	r.UnreadRune()

	switch b {
	case '[':
		return jsonDecodeArray(ctx, r, depth, opt)
	case '{':
		return jsonDecodeObject(ctx, r, depth, opt)
	case '"':
		return jsonDecodeString(ctx, r, depth, opt)
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-':
		return jsonDecodeNumeric(ctx, r, depth, opt)
	case 't', 'T':
		return jsonDecodeExpectValue(ctx, r, "true", core.ZBool(true), depth, opt)
	case 'f', 'F':
		return jsonDecodeExpectValue(ctx, r, "false", core.ZBool(false), depth, opt)
	case 'n', 'N':
		return jsonDecodeExpectValue(ctx, r, "null", core.ZNULL, depth, opt)
	default:
		return nil, ErrSyntax
	}
}

func jsonDecodeObject(ctx core.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	depth -= 1
	if depth <= 0 {
		return nil, ErrDepth
	}

	b, err := nextRune(r)
	if err != nil {
		return nil, err
	}
	if b != '{' {
		return nil, ErrSyntax
	}

	var set func(ctx core.Context, k, v *core.ZVal) error
	var final *core.ZVal

	if opt&ObjectAsArray == ObjectAsArray {
		a := core.NewZArray()
		set = a.OffsetSet
		final = a.ZVal()
	} else {
		o, err := core.NewZObject(ctx, nil) // nil means stdClass
		if err != nil {
			// should never happen for stdClass
			return nil, err
		}
		set = o.ObjectSet
		final = o.ZVal()
	}

	for {
		// remove spaces and check for empty objects
		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b == '}' {
			return final, nil
		}
		r.UnreadRune()

		k, err := jsonDecodeString(ctx, r, depth, opt)
		if err != nil {
			return nil, err
		}

		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b != ':' {
			return nil, ErrSyntax
		}

		z, err := jsonDecodeAny(ctx, r, depth, opt)
		if err != nil {
			return nil, err
		}
		err = set(ctx, k, z)
		if err != nil {
			return nil, err
		}

		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b == ',' {
			continue
		}
		if b == '}' {
			return final, nil
		}
	}
}

func jsonDecodeArray(ctx core.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	depth -= 1
	if depth <= 0 {
		return nil, ErrDepth
	}

	b, err := nextRune(r)
	if err != nil {
		return nil, err
	}
	if b != '[' {
		return nil, ErrSyntax
	}

	a := core.NewZArray()

	for {
		// remove spaces and check for empty arrays/etc
		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b == ']' {
			return a.ZVal(), nil
		}
		r.UnreadRune()

		z, err := jsonDecodeAny(ctx, r, depth, opt)
		if err != nil {
			return nil, err
		}
		err = a.OffsetSet(ctx, nil, z)
		if err != nil {
			return nil, err
		}

		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b == ',' {
			continue
		}
		if b == ']' {
			return a.ZVal(), nil
		}
	}
}

func jsonDecodeString(ctx core.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != '"' {
		return nil, ErrSyntax
	}

	var buf []byte

	for {
		c, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if c == '"' {
			// end of string
			return core.ZString(buf).ZVal(), nil
		}

		if c != '\\' {
			buf = append(buf, c)
			continue
		}

		c, err = r.ReadByte()
		if err != nil {
			return nil, err
		}

		switch c {
		case 'n':
			buf = append(buf, '\n')
		case 'r':
			buf = append(buf, '\r')
		case 't':
			buf = append(buf, '\t')
		case '\\', '/', '"':
			buf = append(buf, c)
		case 'u':
			// unicode
			cp := make([]byte, 4) // 4 bytes length
			_, err = r.Read(cp)
			if err != nil {
				return nil, err
			}
			v, err := strconv.ParseInt(string(cp), 16, 16)
			if err != nil {
				return nil, ErrSyntax
			}
			s := utf8.EncodeRune(cp, rune(v))
			buf = append(buf, cp[:s]...)
		default:
			return nil, ErrSyntax
		}
	}
}

func jsonDecodeNumeric(ctx core.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	// we have a numeric value, read it
	var buf []byte

	p := 0

	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if c >= '0' && c <= '9' {
			if p == 0 || p == 3 {
				p++
			}
			buf = append(buf, c)
			continue
		}
		if c == '+' || c == '-' {
			if p == 0 || p == 3 {
				p++
				buf = append(buf, c)
				continue
			}
			r.UnreadByte()
			break
		}
		if c == '.' {
			if p == 1 {
				p = 2
				buf = append(buf, c)
				continue
			}
			r.UnreadByte()
			break
		}
		if c == 'e' || c == 'E' {
			if p < 3 {
				p = 3
				buf = append(buf, c)
				continue
			}
			r.UnreadByte()
			break
		}
		r.UnreadByte()
		break
	}

	if buf == nil {
		return nil, ErrSyntax
	}

	if p < 1 {
		// int value
		v, err := strconv.ParseInt(string(buf), 10, 64)
		if err == nil {
			return core.ZInt(v).ZVal(), nil
		}
		// too large? check if BigintAsString is set
		if opt&BigintAsString == BigintAsString {
			return core.ZString(buf).ZVal(), nil
		}
		// if not set, attempt to parse as float
	}
	// float
	v, err := strconv.ParseFloat(string(buf), 64)
	if err != nil {
		return nil, err
	}
	return core.ZFloat(v).ZVal(), nil
}

func jsonDecodeExpectValue(ctx core.Context, r *strings.Reader, expect string, value core.Val, depth int, opt JsonDecOpt) (*core.ZVal, error) {
	b := make([]byte, len(expect))
	_, err := r.Read(b)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(string(b)) != expect {
		return nil, ErrSyntax
	}

	return value.ZVal(), nil
}

package json

import (
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed json_decode ( string $json [, bool $assoc = FALSE [, int $depth = 512 [, int $options = 0 ]]] )
func fncJsonDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var json phpv.ZString
	var assoc *phpv.ZBool
	var depth, opt *phpv.ZInt

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

	result, jsonErr := jsonDecodeAny(ctx, strings.NewReader(string(json)), d, o)
	if jsonErr != nil {
		if je, ok := jsonErr.(JsonError); ok {
			setLastJsonError(ctx, je)
			return phpv.ZNULL.ZVal(), nil
		}
		setLastJsonError(ctx, ErrSyntax)
		return phpv.ZNULL.ZVal(), nil
	}
	setLastJsonError(ctx, ErrNone)
	return result, nil
}

func setLastJsonError(ctx phpv.Context, err JsonError) {
	ctx.Global().OffsetSet(ctx, phpv.ZString("__json_last_error"), phpv.ZInt(int64(err)).ZVal())
}

func getLastJsonError(ctx phpv.Context) JsonError {
	v, err := ctx.Global().OffsetGet(ctx, phpv.ZString("__json_last_error"))
	if err != nil || v == nil || v.GetType() == phpv.ZtNull {
		return ErrNone
	}
	return JsonError(v.AsInt(ctx))
}

// > func int json_last_error ( void )
func fncJsonLastError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(int64(getLastJsonError(ctx))).ZVal(), nil
}

// > func string json_last_error_msg ( void )
func fncJsonLastErrorMsg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	err := getLastJsonError(ctx)
	var msg string
	switch err {
	case ErrNone:
		msg = "No error"
	case ErrDepth:
		msg = "Maximum stack depth exceeded"
	case ErrStateMismatch:
		msg = "Underflow or the modes mismatch"
	case ErrCtrlChar:
		msg = "Unexpected control character found"
	case ErrSyntax:
		msg = "Syntax error"
	case ErrUtf8:
		msg = "Malformed UTF-8 characters, possibly incorrectly encoded"
	case ErrRecursion:
		msg = "Recursion detected"
	case ErrInfOrNan:
		msg = "Inf and NaN cannot be JSON encoded"
	case ErrUnsupportedType:
		msg = "Type is not supported"
	case ErrInvalidPropName:
		msg = "The decoded property name is not valid for PHP"
	case ErrUtf16:
		msg = "Single unpaired UTF-16 surrogate in unicode escape"
	case ErrNonBackedEnum:
		msg = "Non-backed enums have no default serialization"
	default:
		msg = "Unknown error"
	}
	return phpv.ZString(msg).ZVal(), nil
}

// > func bool json_validate ( string $json [, int $depth = 512 [, int $flags = 0 ]] )
func fncJsonValidate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var json phpv.ZString
	var depth *phpv.ZInt

	_, err := core.Expand(ctx, args, &json, &depth)
	if err != nil {
		return nil, err
	}

	d := 512
	if depth != nil {
		d = int(*depth)
	}

	_, jsonErr := jsonDecodeAny(ctx, strings.NewReader(string(json)), d, 0)
	if jsonErr != nil {
		if je, ok := jsonErr.(JsonError); ok {
			setLastJsonError(ctx, je)
		} else {
			setLastJsonError(ctx, ErrSyntax)
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	setLastJsonError(ctx, ErrNone)
	return phpv.ZBool(true).ZVal(), nil
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

func jsonDecodeAny(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
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
		return jsonDecodeExpectValue(ctx, r, "true", phpv.ZBool(true), depth, opt)
	case 'f', 'F':
		return jsonDecodeExpectValue(ctx, r, "false", phpv.ZBool(false), depth, opt)
	case 'n', 'N':
		return jsonDecodeExpectValue(ctx, r, "null", phpv.ZNULL, depth, opt)
	default:
		return nil, ErrSyntax
	}
}

func jsonDecodeObject(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
	depth -= 1
	if depth < 0 {
		return nil, ErrDepth
	}

	b, err := nextRune(r)
	if err != nil {
		return nil, err
	}
	if b != '{' {
		return nil, ErrSyntax
	}

	var set func(ctx phpv.Context, k phpv.Val, v *phpv.ZVal) error
	var final *phpv.ZVal

	if opt&ObjectAsArray == ObjectAsArray {
		a := phpv.NewZArray()
		set = a.OffsetSet
		final = a.ZVal()
	} else {
		o, err := phpobj.NewZObject(ctx, nil) // nil means stdClass
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

func jsonDecodeArray(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
	depth -= 1
	if depth < 0 {
		return nil, ErrDepth
	}

	b, err := nextRune(r)
	if err != nil {
		return nil, err
	}
	if b != '[' {
		return nil, ErrSyntax
	}

	a := phpv.NewZArray()

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

func jsonDecodeString(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
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
			return phpv.ZString(buf).ZVal(), nil
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

func jsonDecodeNumeric(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
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

	if p <= 1 {
		// int value (p=0 means no digits yet which shouldn't happen, p=1 means pure integer)
		v, err := strconv.ParseInt(string(buf), 10, 64)
		if err == nil {
			return phpv.ZInt(v).ZVal(), nil
		}
		// too large? check if BigintAsString is set
		if opt&BigintAsString == BigintAsString {
			return phpv.ZString(buf).ZVal(), nil
		}
		// if not set, attempt to parse as float
	}
	// float
	v, err := strconv.ParseFloat(string(buf), 64)
	if err != nil {
		return nil, err
	}
	return phpv.ZFloat(v).ZVal(), nil
}

func jsonDecodeExpectValue(ctx phpv.Context, r *strings.Reader, expect string, value phpv.Val, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
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

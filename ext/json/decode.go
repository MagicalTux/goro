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
	// PHP 8.1+: Passing null to json_decode() is deprecated
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtNull {
		ctx.Deprecated("json_decode(): Passing null to parameter #1 ($json) of type string is deprecated")
	}

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

	// PHP's depth semantics: depth=N allows nesting up to N-1 levels.
	reader := strings.NewReader(string(json))
	result, jsonErr := jsonDecodeAny(ctx, reader, d-1, o)
	if jsonErr != nil {
		if je, ok := jsonErr.(JsonError); ok {
			setLastJsonError(ctx, je)
			return phpv.ZNULL.ZVal(), nil
		}
		setLastJsonError(ctx, ErrSyntax)
		return phpv.ZNULL.ZVal(), nil
	}

	// Check for trailing non-whitespace content (invalid JSON)
	for {
		b, readErr := reader.ReadByte()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			setLastJsonError(ctx, ErrSyntax)
			return phpv.ZNULL.ZVal(), nil
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			setLastJsonError(ctx, ErrSyntax)
			return phpv.ZNULL.ZVal(), nil
		}
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
		msg = "State mismatch (invalid or malformed JSON)"
	case ErrCtrlChar:
		msg = "Control character error, possibly incorrectly encoded"
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

	reader := strings.NewReader(string(json))
	_, jsonErr := jsonDecodeAny(ctx, reader, d-1, 0)
	if jsonErr != nil {
		if je, ok := jsonErr.(JsonError); ok {
			setLastJsonError(ctx, je)
		} else {
			setLastJsonError(ctx, ErrSyntax)
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// Check for trailing non-whitespace content
	for {
		b, readErr := reader.ReadByte()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			setLastJsonError(ctx, ErrSyntax)
			return phpv.ZBool(false).ZVal(), nil
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			setLastJsonError(ctx, ErrSyntax)
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	setLastJsonError(ctx, ErrNone)
	return phpv.ZBool(true).ZVal(), nil
}

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
	case 't':
		return jsonDecodeExpectValue(ctx, r, "true", phpv.ZBool(true), depth, opt)
	case 'f':
		return jsonDecodeExpectValue(ctx, r, "false", phpv.ZBool(false), depth, opt)
	case 'n':
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
		o, err := phpobj.NewZObject(ctx, nil)
		if err != nil {
			return nil, err
		}
		set = o.ObjectSet
		final = o.ZVal()
	}

	for {
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
		return nil, ErrStateMismatch
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
			return nil, ErrSyntax
		}
		if b == ',' {
			continue
		}
		if b == ']' {
			return a.ZVal(), nil
		}
		return nil, ErrStateMismatch
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
			return phpv.ZString(buf).ZVal(), nil
		}

		if c != '\\' {
			if c < 0x20 {
				return nil, ErrCtrlChar
			}
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
		case 'b':
			buf = append(buf, '\b')
		case 'f':
			buf = append(buf, '\f')
		case '\\', '/', '"':
			buf = append(buf, c)
		case 'u':
			cp := make([]byte, 4)
			_, err = io.ReadFull(r, cp)
			if err != nil {
				return nil, err
			}
			v, err := strconv.ParseUint(string(cp), 16, 32)
			if err != nil {
				return nil, ErrSyntax
			}
			codepoint := rune(v)
			// Handle UTF-16 surrogate pairs
			if codepoint >= 0xD800 && codepoint <= 0xDBFF {
				b1, serr := r.ReadByte()
				if serr != nil {
					return nil, ErrUtf8
				}
				b2, serr := r.ReadByte()
				if serr != nil {
					return nil, ErrUtf8
				}
				if b1 != '\\' || b2 != 'u' {
					return nil, ErrUtf8
				}
				cp2 := make([]byte, 4)
				_, serr = io.ReadFull(r, cp2)
				if serr != nil {
					return nil, ErrUtf8
				}
				v2, serr := strconv.ParseUint(string(cp2), 16, 32)
				if serr != nil {
					return nil, ErrSyntax
				}
				lo := rune(v2)
				if lo < 0xDC00 || lo > 0xDFFF {
					return nil, ErrUtf16
				}
				codepoint = 0x10000 + (codepoint-0xD800)*0x400 + (lo - 0xDC00)
			} else if codepoint >= 0xDC00 && codepoint <= 0xDFFF {
				return nil, ErrUtf16
			}
			var ubuf [4]byte
			s := utf8.EncodeRune(ubuf[:], codepoint)
			buf = append(buf, ubuf[:s]...)
		default:
			return nil, ErrSyntax
		}
	}
}

func jsonDecodeNumeric(ctx phpv.Context, r *strings.Reader, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
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
		v, err := strconv.ParseInt(string(buf), 10, 64)
		if err == nil {
			return phpv.ZInt(v).ZVal(), nil
		}
		if opt&BigintAsString == BigintAsString {
			return phpv.ZString(buf).ZVal(), nil
		}
	}
	v, err := strconv.ParseFloat(string(buf), 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			return phpv.ZFloat(v).ZVal(), nil
		}
		return nil, ErrSyntax
	}
	return phpv.ZFloat(v).ZVal(), nil
}

func jsonDecodeExpectValue(ctx phpv.Context, r *strings.Reader, expect string, value phpv.Val, depth int, opt JsonDecOpt) (*phpv.ZVal, error) {
	b := make([]byte, len(expect))
	_, err := r.Read(b)
	if err != nil {
		return nil, err
	}
	if string(b) != expect {
		return nil, ErrSyntax
	}
	return value.ZVal(), nil
}

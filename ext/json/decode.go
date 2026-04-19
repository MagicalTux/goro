package json

import (
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func mixed json_decode ( string $json [, bool $assoc = FALSE [, int $depth = 512 [, int $options = 0 ]]] )
func fncJsonDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// PHP 8.1+: Passing null to json_decode() is deprecated
	if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtNull {
		ctx.Deprecated("Passing null to parameter #1 ($json) of type string is deprecated")
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
	if d <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "json_decode(): Argument #3 ($depth) must be greater than 0")
	}
	if d > 0x7FFFFFFF {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "json_decode(): Argument #3 ($depth) must be less than 2147483648")
	}
	var rawOpt int
	if opt != nil {
		rawOpt = int(*opt)
		o = JsonDecOpt(rawOpt)
	}
	throwOnError := rawOpt&ThrowOnError != 0
	if assoc != nil && *assoc {
		o |= ObjectAsArray
	}

	// Check for invalid UTF-8 in the input and handle based on flags.
	jsonStr := string(json)
	if !utf8.ValidString(jsonStr) {
		if rawOpt&InvalidUtf8Ignore != 0 {
			jsonStr = fixInvalidUtf8(jsonStr, false)
		} else if rawOpt&InvalidUtf8Substitute != 0 {
			jsonStr = fixInvalidUtf8(jsonStr, true)
		} else {
			jsonErrCode := ErrUtf8
			if throwOnError {
				return nil, throwJsonException(ctx, jsonErrCode)
			}
			setLastJsonError(ctx, jsonErrCode)
			return phpv.ZNULL.ZVal(), nil
		}
	}

	// PHP's depth semantics: depth=N allows nesting up to N-1 levels.
	reader := strings.NewReader(jsonStr)
	result, jsonErr := jsonDecodeAny(ctx, reader, d-1, o)
	if jsonErr != nil {
		jsonErrCode := ErrSyntax
		if je, ok := jsonErr.(JsonError); ok {
			jsonErrCode = je
		}
		if throwOnError {
			return nil, throwJsonException(ctx, jsonErrCode)
		}
		setLastJsonError(ctx, jsonErrCode)
		return phpv.ZNULL.ZVal(), nil
	}

	// Check for trailing non-whitespace content (invalid JSON)
	for {
		b, readErr := reader.ReadByte()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if throwOnError {
				return nil, throwJsonException(ctx, ErrSyntax)
			}
			setLastJsonError(ctx, ErrSyntax)
			return phpv.ZNULL.ZVal(), nil
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			if throwOnError {
				return nil, throwJsonException(ctx, ErrSyntax)
			}
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

// throwJsonException throws a JsonException with the given error code and message.
// This does NOT modify the global json_last_error state (per PHP spec).
func throwJsonException(ctx phpv.Context, err JsonError) error {
	msg := jsonErrorMessage(err)
	return phpobj.ThrowErrorCode(ctx, JsonException, int(err), msg)
}

func jsonErrorMessage(err JsonError) string {
	switch err {
	case ErrNone:
		return "No error"
	case ErrDepth:
		return "Maximum stack depth exceeded"
	case ErrStateMismatch:
		return "State mismatch (invalid or malformed JSON)"
	case ErrCtrlChar:
		return "Control character error, possibly incorrectly encoded"
	case ErrSyntax:
		return "Syntax error"
	case ErrUtf8:
		return "Malformed UTF-8 characters, possibly incorrectly encoded"
	case ErrRecursion:
		return "Recursion detected"
	case ErrInfOrNan:
		return "Inf and NaN cannot be JSON encoded"
	case ErrUnsupportedType:
		return "Type is not supported"
	case ErrInvalidPropName:
		return "The decoded property name is invalid"
	case ErrUtf16:
		return "Single unpaired UTF-16 surrogate in unicode escape"
	case ErrNonBackedEnum:
		return "Non-backed enums have no default serialization"
	default:
		return "Unknown error"
	}
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
		msg = "The decoded property name is invalid"
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
	var depth, flags *phpv.ZInt

	_, err := core.Expand(ctx, args, &json, &depth, &flags)
	if err != nil {
		return nil, err
	}

	d := 512
	if depth != nil {
		d = int(*depth)
	}
	if d == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "json_validate(): Argument #2 ($depth) must be greater than 0")
	}
	if d > 0x7FFFFFFF {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "json_validate(): Argument #2 ($depth) must be less than 2147483648")
	}

	// Validate flags: only JSON_INVALID_UTF8_IGNORE is allowed
	var flagVal int
	if flags != nil {
		flagVal = int(*flags)
	}
	if flagVal != 0 && flagVal != InvalidUtf8Ignore {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "json_validate(): Argument #3 ($flags) must be a valid flag (allowed flags: JSON_INVALID_UTF8_IGNORE)")
	}

	// Check for invalid UTF-8 in the input.
	jsonStr := string(json)
	if !utf8.ValidString(jsonStr) {
		if flagVal == InvalidUtf8Ignore {
			jsonStr = fixInvalidUtf8(jsonStr, false)
		} else {
			setLastJsonError(ctx, ErrUtf8)
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	reader := strings.NewReader(jsonStr)
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
		if b >= 0 && b < 0x20 {
			return nil, ErrCtrlChar
		}
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
	var final func() *phpv.ZVal

	if opt&ObjectAsArray == ObjectAsArray {
		a := phpv.NewZArray()
		set = a.OffsetSet
		final = a.ZVal
	} else {
		// Lazily create the stdClass object so that object IDs are not
		// consumed when parsing fails (e.g. incomplete JSON input).
		var o *phpobj.ZObject
		set = func(ctx phpv.Context, k phpv.Val, v *phpv.ZVal) error {
			if o == nil {
				var err error
				o, err = phpobj.NewZObject(ctx, nil)
				if err != nil {
					return err
				}
			}
			return o.ObjectSet(ctx, k, v)
		}
		final = func() *phpv.ZVal {
			if o == nil {
				// Empty object: create it now
				var err error
				o, err = phpobj.NewZObject(ctx, nil)
				if err != nil {
					return phpv.ZNULL.ZVal()
				}
			}
			return o.ZVal()
		}
	}

	for {
		b, err = nextRune(r)
		if err != nil {
			return nil, err
		}
		if b == '}' {
			return final(), nil
		}
		r.UnreadRune()

		k, err := jsonDecodeString(ctx, r, depth, opt)
		if err != nil {
			return nil, err
		}

		// PHP rejects property names containing null bytes
		if kStr, ok := k.Value().(phpv.ZString); ok && strings.Contains(string(kStr), "\x00") {
			return nil, ErrInvalidPropName
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
			// Check for trailing comma: peek at next non-whitespace character
			next, peekErr := nextRune(r)
			if peekErr != nil {
				return nil, ErrSyntax
			}
			if next == '}' {
				// Trailing comma is not valid JSON
				return nil, ErrSyntax
			}
			r.UnreadRune()
			continue
		}
		if b == '}' {
			return final(), nil
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
			// Check for trailing comma: peek at next non-whitespace character
			next, peekErr := nextRune(r)
			if peekErr != nil {
				return nil, ErrSyntax
			}
			if next == ']' {
				// Trailing comma is not valid JSON
				return nil, ErrSyntax
			}
			r.UnreadRune()
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
					return nil, ErrUtf16
				}
				b2, serr := r.ReadByte()
				if serr != nil {
					return nil, ErrUtf16
				}
				if b1 != '\\' || b2 != 'u' {
					return nil, ErrUtf16
				}
				cp2 := make([]byte, 4)
				_, serr = io.ReadFull(r, cp2)
				if serr != nil {
					return nil, ErrUtf16
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
			// Reject leading zeros: if we just appended '0' as the first digit
			// (or after a '-' sign), check if the next character is also a digit.
			// If so, stop here so the extra digits become trailing garbage.
			if p == 1 {
				hasLeadingZero := (len(buf) == 1 && buf[0] == '0') ||
					(len(buf) == 2 && buf[0] == '-' && buf[1] == '0')
				if hasLeadingZero {
					// Peek at the next byte
					next, peekErr := r.ReadByte()
					if peekErr == nil {
						if next >= '0' && next <= '9' {
							// Leading zero followed by digit: stop, unread the digit.
							r.UnreadByte()
							break
						}
						r.UnreadByte()
					}
				}
			}
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

// fixInvalidUtf8 processes a string that contains invalid UTF-8 sequences.
// If substitute is true, each invalid byte is replaced with U+FFFD (UTF-8: 0xEF 0xBF 0xBD).
// If substitute is false, invalid bytes are dropped (ignored).
// PHP processes each invalid byte individually (one U+FFFD per invalid byte for SUBSTITUTE,
// each byte dropped separately for IGNORE).
func fixInvalidUtf8(s string, substitute bool) string {
	var buf []byte
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != utf8.RuneError || size != 1 {
			// Valid rune (size >= 1 for ASCII, size >= 2 for multi-byte valid runes).
			// Also handles actual U+FFFD (size == 3).
			buf = append(buf, s[i:i+size]...)
			i += size
			continue
		}
		// Invalid UTF-8 byte: process byte-by-byte.
		if substitute {
			buf = append(buf, 0xEF, 0xBF, 0xBD) // U+FFFD
		}
		// Drop or substitute one byte at a time (not the whole sequence unit).
		i++
	}
	return string(buf)
}

// invalidUtf8SeqLen returns the number of bytes in an invalid UTF-8 sequence
// starting at position i in s. It returns at least 1.
func invalidUtf8SeqLen(s string, i int) int {
	if i >= len(s) {
		return 1
	}
	b := s[i]
	// Determine expected sequence length from the start byte
	var expectedLen int
	switch {
	case b < 0x80:
		// ASCII byte that failed decoding (shouldn't happen normally)
		expectedLen = 1
	case b < 0xC0:
		// Lone continuation byte
		expectedLen = 1
	case b < 0xE0:
		expectedLen = 2
	case b < 0xF0:
		expectedLen = 3
	default:
		expectedLen = 4
	}
	// Count how many valid continuation bytes (0x80-0xBF) follow
	n := 1
	for n < expectedLen && i+n < len(s) {
		cb := s[i+n]
		if cb >= 0x80 && cb < 0xC0 {
			n++
		} else {
			break
		}
	}
	return n
}

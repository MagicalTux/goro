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
	st := &jsonState{}
	return appendJsonEncodeState(ctx, r, v, opt, depth, st)
}

func appendJsonEncodeState(ctx phpv.Context, r []byte, v *phpv.ZVal, opt JsonEncOpt, depth int, st *jsonState) ([]byte, error) {
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
		p := phpv.GetSerializePrecision(ctx)
		s := strconv.FormatFloat(float64(v.Value().(phpv.ZFloat)), 'g', p, 64)
		return append(r, []byte(s)...), nil
	case phpv.ZtString:
		return appendJsonString(r, string(v.Value().(phpv.ZString)), opt)
	case phpv.ZtArray:
		a := v.Value().(*phpv.ZArray)
		if a.HasStringKeys() || (opt&ForceObject != 0 && a.Count(ctx) > 0) {
			return appendJsonObject(ctx, r, a.NewIterator(), opt, depth, st)
		} else {
			return appendJsonArray(ctx, r, a.NewIterator(), opt, depth, st)
		}
	case phpv.ZtObject:
		// TODO check for JsonSerializable
		it := v.NewIterator()
		if it == nil {
			return r, ErrUnsupportedType
		}
		return appendJsonObject(ctx, r, it, opt, depth, st)
	default:
		return r, ErrUnsupportedType
	}
}

// jsonIndent returns the indentation string for the given indent level.
func jsonIndent(level int) []byte {
	if level <= 0 {
		return nil
	}
	s := make([]byte, level*4)
	for i := range s {
		s[i] = ' '
	}
	return s
}

// jsonState carries encoding state including indent level for pretty printing
type jsonState struct {
	indent int
}

func appendJsonArray(ctx phpv.Context, r []byte, it phpv.ZIterator, opt JsonEncOpt, depth int, st *jsonState) ([]byte, error) {
	depth = depth - 1
	if depth <= 0 {
		return r, ErrDepth
	}
	pretty := opt&PrettyPrint != 0
	r = append(r, '[')
	first := true

	oldIndent := st.indent
	st.indent++

	for ; it.Valid(ctx); it.Next(ctx) {
		v, err := it.Current(ctx)
		if err != nil {
			return r, err
		}

		if !first {
			r = append(r, ',')
		}
		first = false

		if pretty {
			r = append(r, '\n')
			r = append(r, jsonIndent(st.indent)...)
		}

		r, err = appendJsonEncodeState(ctx, r, v, opt, depth, st)
		if err != nil {
			return r, err
		}
	}
	st.indent = oldIndent
	if pretty && !first {
		r = append(r, '\n')
		r = append(r, jsonIndent(st.indent)...)
	}
	r = append(r, ']')
	return r, nil
}

func appendJsonObject(ctx phpv.Context, r []byte, it phpv.ZIterator, opt JsonEncOpt, depth int, st *jsonState) ([]byte, error) {
	depth = depth - 1
	if depth <= 0 {
		return r, ErrDepth
	}
	pretty := opt&PrettyPrint != 0
	r = append(r, '{')
	first := true

	oldIndent := st.indent
	st.indent++

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

		if pretty {
			r = append(r, '\n')
			r = append(r, jsonIndent(st.indent)...)
		}

		r, err = appendJsonString(r, string(k.Value().(phpv.ZString)), opt)
		if err != nil {
			return r, err
		}
		if pretty {
			r = append(r, ':', ' ')
		} else {
			r = append(r, ':')
		}

		r, err = appendJsonEncodeState(ctx, r, v, opt, depth, st)
		if err != nil {
			return r, err
		}
	}
	st.indent = oldIndent
	if pretty && !first {
		r = append(r, '\n')
		r = append(r, jsonIndent(st.indent)...)
	}
	r = append(r, '}')
	return r, nil
}

func appendJsonString(r []byte, s string, opt JsonEncOpt) ([]byte, error) {
	unescSlash := opt&UnescapedSlashes != 0
	unescUnicode := opt&UnescapedUnicode != 0
	hexTag := opt&HexTag != 0
	hexAmp := opt&HexAmp != 0
	hexApos := opt&HexApos != 0
	hexQuot := opt&HexQuot != 0

	r = append(r, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf { // ASCII
			needsEscape := false
			switch b {
			case '"':
				needsEscape = true
			case '/':
				needsEscape = !unescSlash
			case '\\':
				needsEscape = true
			case '<':
				needsEscape = hexTag
			case '>':
				needsEscape = hexTag
			case '&':
				needsEscape = hexAmp
			case '\'':
				needsEscape = hexApos
			default:
				if b < 0x20 {
					needsEscape = true
				}
			}

			if !needsEscape {
				i++
				continue
			}

			if start < i {
				r = append(r, []byte(s[start:i])...)
			}

			// escape the character
			switch b {
			case '"':
				if hexQuot {
					r = append(r, []byte(`\u0022`)...)
				} else {
					r = append(r, '\\', '"')
				}
			case '/':
				r = append(r, '\\', '/')
			case '\\':
				r = append(r, '\\', '\\')
			case '<':
				r = append(r, []byte(`\u003C`)...)
			case '>':
				r = append(r, []byte(`\u003E`)...)
			case '&':
				r = append(r, []byte(`\u0026`)...)
			case '\'':
				r = append(r, []byte(`\u0027`)...)
			case '\n':
				r = append(r, '\\', 'n')
			case '\r':
				r = append(r, '\\', 'r')
			case '\t':
				r = append(r, '\\', 't')
			case '\b':
				r = append(r, '\\', 'b')
			case '\f':
				r = append(r, '\\', 'f')
			default:
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
				r = append(r, []byte(`\ufffd`)...)
			} else if opt&InvalidUtf8Ignore == 0 {
				return r, ErrUtf8
			}
			i += size
			start = i
			continue
		}

		// Escape non-ASCII if JSON_UNESCAPED_UNICODE is not set
		if !unescUnicode {
			if start < i {
				r = append(r, []byte(s[start:i])...)
			}
			if c <= 0xFFFF {
				r = append(r, '\\', 'u',
					hex[(c>>12)&0xf], hex[(c>>8)&0xf],
					hex[(c>>4)&0xf], hex[c&0xf])
			} else {
				// Encode as UTF-16 surrogate pair
				c -= 0x10000
				hi := 0xD800 + (c>>10)&0x3FF
				lo := 0xDC00 + c&0x3FF
				r = append(r, '\\', 'u',
					hex[(hi>>12)&0xf], hex[(hi>>8)&0xf],
					hex[(hi>>4)&0xf], hex[hi&0xf])
				r = append(r, '\\', 'u',
					hex[(lo>>12)&0xf], hex[(lo>>8)&0xf],
					hex[(lo>>4)&0xf], hex[lo&0xf])
			}
			i += size
			start = i
			continue
		}

		// U+2028 LINE SEPARATOR, U+2029 PARAGRAPH SEPARATOR
		// Escape these unless JSON_UNESCAPED_LINE_TERMINATORS is set
		if (c == '\u2028' || c == '\u2029') && opt&UnescapedEOL == 0 {
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

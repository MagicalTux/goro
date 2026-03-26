package json

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
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

	st := &jsonState{partialOutput: o&PartialOutputOnError != 0}
	r, jsonErr := appendJsonEncodeState(ctx, r, v, o, d, st)

	if jsonErr != nil {
		if je, ok := jsonErr.(JsonError); ok {
			if o&JsonEncOpt(ThrowOnError) != 0 {
				// JSON_THROW_ON_ERROR: throw exception without modifying global error state
				return nil, throwJsonException(ctx, je)
			}
			setLastJsonError(ctx, je)
			if o&PartialOutputOnError != 0 {
				// For partial output, return "null" for top-level encoding failures.
				// The partial buffer from array/object sub-elements is handled
				// inside appendJsonArray/appendJsonObject. Here at the top level
				// we always substitute "null" for the failing value.
				return phpv.ZString("null").ZVal(), nil
			}
			return phpv.ZBool(false).ZVal(), nil
		}
		setLastJsonError(ctx, ErrUnsupportedType)
		if o&PartialOutputOnError != 0 {
			return phpv.ZString("null").ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}
	if st.lastError != ErrNone {
		setLastJsonError(ctx, st.lastError)
	} else {
		setLastJsonError(ctx, ErrNone)
	}
	return phpv.ZString(r).ZVal(), nil
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
		f := float64(v.Value().(phpv.ZFloat))
		if math.IsInf(f, 0) || math.IsNaN(f) {
			if st.partialOutput {
				st.lastError = ErrInfOrNan
				return append(r, '0'), nil
			}
			return r, ErrInfOrNan
		}
		s := formatJsonFloat(ctx, f, opt)
		return append(r, []byte(s)...), nil
	case phpv.ZtString:
		s := v.Value().(phpv.ZString)
		if opt&NumericCheck != 0 && s.IsNumeric() {
			numVal, err := s.AsNumeric()
			if err == nil {
				switch nv := numVal.(type) {
				case phpv.ZInt:
					return append(r, []byte(strconv.FormatInt(int64(nv), 10))...), nil
				case phpv.ZFloat:
					fs := formatJsonFloat(ctx, float64(nv), opt)
					return append(r, []byte(fs)...), nil
				}
			}
		}
		return appendJsonString(r, string(s), opt)
	case phpv.ZtArray:
		a := v.Value().(*phpv.ZArray)
		if st.markArray(a) {
			return r, ErrRecursion
		}
		defer st.unmarkArray(a)
		if opt&ForceObject != 0 || !isJsonList(ctx, a) {
			return appendJsonObject(ctx, r, a.NewIterator(), opt, depth, st)
		} else {
			return appendJsonArray(ctx, r, a.NewIterator(), opt, depth, st)
		}
	case phpv.ZtObject:
		obj := v.AsObject(ctx)
		if obj == nil {
			return r, ErrUnsupportedType
		}
		// Lazy objects: json_encode triggers initialization
		if zo, ok := obj.(*phpobj.ZObject); ok && zo.IsLazy() {
			if err := zo.TriggerLazyInit(ctx); err != nil {
				return r, err
			}
		}
		// For initialized proxies, use the real instance
		if zo, ok := obj.(*phpobj.ZObject); ok && zo.LazyState == phpobj.LazyProxyInitialized && zo.LazyInstance != nil {
			obj = zo.LazyInstance
		}
		if st.markObject(obj) {
			return r, ErrRecursion
		}
		defer st.unmarkObject(obj)
		if g, ok := ctx.Global().(*phpctx.Global); ok {
			if g.MarkJsonEncoding(obj) {
				return r, ErrRecursion
			}
			defer g.UnmarkJsonEncoding(obj)
		}
		if obj.GetClass().GetType().Has(phpv.ZClassTypeEnum) {
			if obj.GetClass().Implements(JsonSerializable) {
				if m, ok := obj.GetClass().GetMethod("jsonserialize"); ok {
					result, err := ctx.CallZVal(ctx, m.Method, nil, obj)
					if err != nil {
						return r, err
					}
					return appendJsonEncodeState(ctx, r, result, opt, depth, st)
				}
			}
			if zc, ok := obj.GetClass().(*phpobj.ZClass); ok && zc.EnumBackingType != 0 {
				if zobj, ok2 := obj.(*phpobj.ZObject); ok2 {
					backingVal := zobj.HashTable().GetString("value")
					if backingVal != nil {
						return appendJsonEncodeState(ctx, r, backingVal, opt, depth, st)
					}
				}
			}
			return r, ErrNonBackedEnum
		}
		if obj.GetClass().Implements(JsonSerializable) {
			if m, ok := obj.GetClass().GetMethod("jsonserialize"); ok {
				result, err := ctx.CallZVal(ctx, m.Method, nil, obj)
				if err != nil {
					return r, err
				}
				return appendJsonEncodeState(ctx, r, result, opt, depth, st)
			}
		}
		it := v.NewIterator()
		if it == nil {
			return r, ErrUnsupportedType
		}
		return appendJsonObject(ctx, r, it, opt, depth, st)
	default:
		return r, ErrUnsupportedType
	}
}

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

type jsonState struct {
	indent        int
	seen          map[phpv.ZObject]bool
	seenArrays    map[*phpv.ZArray]bool
	partialOutput bool
	lastError     JsonError
}

func (st *jsonState) markObject(obj phpv.ZObject) bool {
	if st.seen == nil {
		st.seen = make(map[phpv.ZObject]bool)
	}
	if st.seen[obj] {
		return true
	}
	st.seen[obj] = true
	return false
}

func (st *jsonState) unmarkObject(obj phpv.ZObject) {
	if st.seen != nil {
		delete(st.seen, obj)
	}
}

func (st *jsonState) markArray(a *phpv.ZArray) bool {
	if st.seenArrays == nil {
		st.seenArrays = make(map[*phpv.ZArray]bool)
	}
	if st.seenArrays[a] {
		return true
	}
	st.seenArrays[a] = true
	return false
}

func (st *jsonState) unmarkArray(a *phpv.ZArray) {
	if st.seenArrays != nil {
		delete(st.seenArrays, a)
	}
}

func formatJsonFloat(ctx phpv.Context, f float64, opt JsonEncOpt) string {
	p := phpv.GetSerializePrecision(ctx)
	var s string
	if p == -1 {
		s = strconv.FormatFloat(f, 'g', -1, 64)
	} else {
		s = strconv.FormatFloat(f, 'g', p, 64)
	}
	if opt&PreserveZeroFraction != 0 {
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
	}
	return s
}

// isJsonList checks if an array should be encoded as a JSON array (list).
// An array is a list if it has sequential integer keys starting from 0.
func isJsonList(ctx phpv.Context, a *phpv.ZArray) bool {
	if a.HasStringKeys() {
		return false
	}
	expectedKey := phpv.ZInt(0)
	it := a.NewIterator()
	for it.Valid(ctx) {
		k, err := it.Key(ctx)
		if err != nil {
			return false
		}
		if k.GetType() != phpv.ZtInt || k.Value().(phpv.ZInt) != expectedKey {
			return false
		}
		expectedKey++
		it.Next(ctx)
	}
	return true
}

func appendJsonArray(ctx phpv.Context, r []byte, it phpv.ZIterator, opt JsonEncOpt, depth int, st *jsonState) ([]byte, error) {
	depth = depth - 1
	if depth < 0 {
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
			if st.partialOutput {
				if je, ok := err.(JsonError); ok {
					st.lastError = je
				}
				r = append(r, []byte("null")...)
			} else {
				return r, err
			}
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
	if depth < 0 {
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
			if st.partialOutput {
				if je, ok := err.(JsonError); ok {
					st.lastError = je
				}
				r = append(r, []byte("null")...)
			} else {
				return r, err
			}
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
		if b := s[i]; b < utf8.RuneSelf {
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
		if !unescUnicode {
			if start < i {
				r = append(r, []byte(s[start:i])...)
			}
			if c <= 0xFFFF {
				r = append(r, '\\', 'u',
					hex[(c>>12)&0xf], hex[(c>>8)&0xf],
					hex[(c>>4)&0xf], hex[c&0xf])
			} else {
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

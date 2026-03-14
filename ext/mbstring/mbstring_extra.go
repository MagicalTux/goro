package mbstring

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// mb_convert_case converts a string using the specified case mode.
// mode: MB_CASE_UPPER (0), MB_CASE_LOWER (1), MB_CASE_TITLE (2)
func fncMbConvertCase(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var mode phpv.ZInt
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &mode, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	switch mode {
	case 0: // MB_CASE_UPPER
		return phpv.ZString(strings.ToUpper(str)).ZVal(), nil
	case 1: // MB_CASE_LOWER
		return phpv.ZString(strings.ToLower(str)).ZVal(), nil
	case 2: // MB_CASE_TITLE
		return phpv.ZString(toTitleCase(str)).ZVal(), nil
	default:
		return phpv.ZFalse.ZVal(), nil
	}
}

// toTitleCase converts a string to title case (first letter of each word uppercased).
func toTitleCase(s string) string {
	prev := ' ' // treat start as after a space
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(rune(prev)) || unicode.IsPunct(rune(prev)) {
			prev = r
			return unicode.ToTitle(r)
		}
		prev = r
		return unicode.ToLower(r)
	}, s)
}

// mb_strstr finds the first occurrence of needle in haystack and returns
// the portion of haystack from the first occurrence of needle to the end.
// If beforeNeedle is true, returns the portion before the first occurrence.
func fncMbStrstr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var beforeNeedle *phpv.ZBool
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle, &beforeNeedle, &enc)
	if err != nil {
		return nil, err
	}

	before := core.Deref(beforeNeedle, false)

	h := string(haystack)
	n := string(needle)

	idx := strings.Index(h, n)
	if idx < 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	if bool(before) {
		return phpv.ZString(h[:idx]).ZVal(), nil
	}
	return phpv.ZString(h[idx:]).ZVal(), nil
}

// mb_stristr is the case-insensitive version of mb_strstr.
func fncMbStristr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var beforeNeedle *phpv.ZBool
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle, &beforeNeedle, &enc)
	if err != nil {
		return nil, err
	}

	before := core.Deref(beforeNeedle, false)

	h := string(haystack)
	n := string(needle)

	hLower := strings.ToLower(h)
	nLower := strings.ToLower(n)

	idx := strings.Index(hLower, nLower)
	if idx < 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	if bool(before) {
		return phpv.ZString(h[:idx]).ZVal(), nil
	}
	return phpv.ZString(h[idx:]).ZVal(), nil
}

// mb_list_encodings returns an array of all supported encodings.
func fncMbListEncodings(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	arr := phpv.NewZArray()
	for _, enc := range supportedEncodings {
		arr.OffsetSet(ctx, nil, phpv.ZString(enc).ZVal())
	}
	return arr.ZVal(), nil
}

// mb_preferred_mime_name returns the MIME charset string for the given encoding.
func fncMbPreferredMimeName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var encoding phpv.ZString

	_, err := core.Expand(ctx, args, &encoding)
	if err != nil {
		return nil, err
	}

	enc := strings.ToUpper(string(encoding))

	mime, ok := encodingToMime[enc]
	if !ok {
		// Try a few normalizations
		normalized := strings.ReplaceAll(enc, "-", "")
		normalized = strings.ReplaceAll(normalized, "_", "")
		for k, v := range encodingToMime {
			kn := strings.ReplaceAll(k, "-", "")
			kn = strings.ReplaceAll(kn, "_", "")
			if kn == normalized {
				return phpv.ZString(v).ZVal(), nil
			}
		}
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(mime).ZVal(), nil
}

// mb_ord returns the Unicode code point of the first character of the string.
func fncMbOrd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	if len(str) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	r, _ := utf8.DecodeRuneInString(str)
	if r == utf8.RuneError {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(r).ZVal(), nil
}

// mb_chr returns the character corresponding to the given Unicode code point.
func fncMbChr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var codepoint phpv.ZInt
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &codepoint, &enc)
	if err != nil {
		return nil, err
	}

	r := rune(codepoint)
	if !utf8.ValidRune(r) {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(string(r)).ZVal(), nil
}

// supportedEncodings lists the encodings we claim to support.
// In practice goro works with UTF-8 internally, but PHP's mb_list_encodings
// returns a broad list. We list the most common ones.
var supportedEncodings = []string{
	"ASCII",
	"UTF-8",
	"UTF-16",
	"UTF-16BE",
	"UTF-16LE",
	"UTF-32",
	"UTF-32BE",
	"UTF-32LE",
	"ISO-8859-1",
	"ISO-8859-2",
	"ISO-8859-3",
	"ISO-8859-4",
	"ISO-8859-5",
	"ISO-8859-6",
	"ISO-8859-7",
	"ISO-8859-8",
	"ISO-8859-9",
	"ISO-8859-10",
	"ISO-8859-13",
	"ISO-8859-14",
	"ISO-8859-15",
	"ISO-8859-16",
	"EUC-JP",
	"SJIS",
	"CP932",
	"EUC-KR",
	"Big5",
	"GB18030",
	"GBK",
	"HZ",
	"KOI8-R",
	"KOI8-U",
	"Windows-1250",
	"Windows-1251",
	"Windows-1252",
	"Windows-1253",
	"Windows-1254",
	"Windows-1255",
	"Windows-1256",
	"Windows-1257",
	"Windows-1258",
}

// encodingToMime maps encoding names (uppercased) to their MIME preferred names.
var encodingToMime = map[string]string{
	"ASCII":        "US-ASCII",
	"UTF-8":        "UTF-8",
	"UTF8":         "UTF-8",
	"UTF-16":       "UTF-16",
	"UTF-16BE":     "UTF-16BE",
	"UTF-16LE":     "UTF-16LE",
	"UTF-32":       "UTF-32",
	"UTF-32BE":     "UTF-32BE",
	"UTF-32LE":     "UTF-32LE",
	"ISO-8859-1":   "ISO-8859-1",
	"ISO-8859-2":   "ISO-8859-2",
	"ISO-8859-3":   "ISO-8859-3",
	"ISO-8859-4":   "ISO-8859-4",
	"ISO-8859-5":   "ISO-8859-5",
	"ISO-8859-6":   "ISO-8859-6",
	"ISO-8859-7":   "ISO-8859-7",
	"ISO-8859-8":   "ISO-8859-8",
	"ISO-8859-9":   "ISO-8859-9",
	"ISO-8859-10":  "ISO-8859-10",
	"ISO-8859-13":  "ISO-8859-13",
	"ISO-8859-14":  "ISO-8859-14",
	"ISO-8859-15":  "ISO-8859-15",
	"ISO-8859-16":  "ISO-8859-16",
	"EUC-JP":       "EUC-JP",
	"EUCJP":        "EUC-JP",
	"SJIS":         "Shift_JIS",
	"SHIFT_JIS":    "Shift_JIS",
	"CP932":        "Shift_JIS",
	"EUC-KR":       "EUC-KR",
	"EUCKR":        "EUC-KR",
	"BIG5":         "Big5",
	"BIG-5":        "Big5",
	"GB18030":      "GB18030",
	"GBK":          "GBK",
	"HZ":           "HZ-GB-2312",
	"KOI8-R":       "KOI8-R",
	"KOI8-U":       "KOI8-U",
	"WINDOWS-1250": "windows-1250",
	"WINDOWS-1251": "windows-1251",
	"WINDOWS-1252": "windows-1252",
	"WINDOWS-1253": "windows-1253",
	"WINDOWS-1254": "windows-1254",
	"WINDOWS-1255": "windows-1255",
	"WINDOWS-1256": "windows-1256",
	"WINDOWS-1257": "windows-1257",
	"WINDOWS-1258": "windows-1258",
}

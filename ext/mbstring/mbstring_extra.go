package mbstring

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// Case mode constants matching PHP 8.1+
const (
	mbCaseUpper       = 0
	mbCaseLower       = 1
	mbCaseTitle       = 2
	mbCaseFold        = 3
	mbCaseUpperSimple = 4
	mbCaseLowerSimple = 5
	mbCaseTitleSimple = 6
	mbCaseFoldSimple  = 7
)

// isCaseIgnorable returns true if the rune is "case-ignorable" per Unicode.
// This includes apostrophes, periods, colons, combining marks, etc.
// Used for Greek final sigma context detection and title case word boundaries.
func isCaseIgnorable(r rune) bool {
	// Common case-ignorable punctuation
	if r == '\'' || r == '\u2019' || r == '\u2018' || // apostrophes
		r == '.' || r == ':' || r == ';' || r == ',' || r == '!' || r == '?' || // basic punct
		r == '\u00B7' || // middle dot
		r == '\u0027' || r == '\u002E' { // ASCII
		return true
	}
	// Unicode general categories: Mn, Me, Cf, Lm, Sk
	if unicode.Is(unicode.Mn, r) || // nonspacing mark
		unicode.Is(unicode.Me, r) || // enclosing mark
		unicode.Is(unicode.Cf, r) || // format
		unicode.Is(unicode.Lm, r) || // modifier letter
		unicode.Is(unicode.Sk, r) { // modifier symbol
		return true
	}
	return false
}

// isCased returns true if the rune has the Cased property (letter that has case).
func isCased(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r)
}

// isWordInternalPunct returns true if the character should be treated as word-internal
// (i.e. does NOT start a new word for title case purposes).
// This includes apostrophes (straight and curly) and similar word-internal punctuation.
func isWordInternalPunct(r rune) bool {
	return r == '\'' || r == '\u2019' || r == '\u2018' || // apostrophes
		r == '\u02BC' || r == '\u02BB' || // modifier letter apostrophe, turned comma
		r == '\u00B7' // middle dot (e.g. Catalan l-geminate)
}

// shouldBeFinalSigma determines if the sigma at position idx in runes should be final sigma.
// PHP uses a context window: scan back up to 63 characters for a cased letter (skipping case-ignorable),
// and scan forward with no limit.
func shouldBeFinalSigma(runes []rune, idx int) bool {
	// Must be preceded by a cased letter (possibly with case-ignorable in between)
	// PHP scans back at least 63 characters (so up to 64 positions back)
	foundCasedBefore := false
	limit := 64
	for i := idx - 1; i >= 0 && (idx-i) <= limit; i-- {
		if isCaseIgnorable(runes[i]) {
			continue
		}
		if isCased(runes[i]) {
			foundCasedBefore = true
		}
		break
	}
	if !foundCasedBefore {
		return false
	}
	// Must NOT be followed by a cased letter (possibly with case-ignorable in between)
	for i := idx + 1; i < len(runes); i++ {
		if isCaseIgnorable(runes[i]) {
			continue
		}
		if isCased(runes[i]) {
			return false
		}
		break
	}
	return true
}

// Full case mapping for upper: ß -> SS, ﬀ -> FF, etc.
func fullToUpper(r rune) string {
	switch r {
	case '\u00DF': // ß -> SS
		return "SS"
	case '\uFB00': // ﬀ -> FF
		return "FF"
	case '\uFB01': // ﬁ -> FI
		return "FI"
	case '\uFB02': // ﬂ -> FL
		return "FL"
	case '\uFB03': // ﬃ -> FFI
		return "FFI"
	case '\uFB04': // ﬄ -> FFL
		return "FFL"
	case '\uFB05': // ﬅ -> ST
		return "ST"
	case '\uFB06': // ﬆ -> ST
		return "ST"
	case '\u0587': // Armenian ew -> ԵՒ
		return "\u0535\u0552"
	case '\u1E9E': // Capital sharp S -> SS
		return "SS"
	// Greek with ypogegrammeni/prosgegrammeni -> uppercase + Iota
	case '\u1FB3': // ᾳ -> ΑΙ
		return "\u0391\u0399"
	case '\u1FBC': // ᾼ -> ΑΙ
		return "\u0391\u0399"
	case '\u1FC3': // ῃ -> ΗΙ
		return "\u0397\u0399"
	case '\u1FCC': // ῌ -> ΗΙ
		return "\u0397\u0399"
	case '\u1FF3': // ῳ -> ΩΙ
		return "\u03A9\u0399"
	case '\u1FFC': // ῼ -> ΩΙ
		return "\u03A9\u0399"
	default:
		return string(unicode.ToUpper(r))
	}
}

// Full case mapping for lower: İ -> i + combining dot above
func fullToLower(r rune, runes []rune, idx int) string {
	switch r {
	case '\u0130': // İ -> i + combining dot above (U+0307)
		return "i\u0307"
	case '\u03A3': // Greek capital sigma -> context-dependent
		if shouldBeFinalSigma(runes, idx) {
			return "\u03C2" // final sigma ς
		}
		return "\u03C3" // small sigma σ
	default:
		return string(unicode.ToLower(r))
	}
}

// Full case fold: ß -> ss, ﬀ -> ff, İ -> i + combining dot above
func fullCaseFold(r rune) string {
	switch r {
	case '\u00DF': // ß -> ss
		return "ss"
	case '\u1E9E': // Capital sharp S -> ss
		return "ss"
	case '\u0130': // İ -> i + combining dot above
		return "i\u0307"
	case '\uFB00': // ﬀ -> ff
		return "ff"
	case '\uFB01': // ﬁ -> fi
		return "fi"
	case '\uFB02': // ﬂ -> fl
		return "fl"
	case '\uFB03': // ﬃ -> ffi
		return "ffi"
	case '\uFB04': // ﬄ -> ffl
		return "ffl"
	case '\uFB05': // ﬅ -> st
		return "st"
	case '\uFB06': // ﬆ -> st
		return "st"
	// Greek with prosgegrammeni -> lowercase + iota
	case '\u1FBC': // ᾼ -> ᾳ
		return "\u1FB3"
	case '\u1FCC': // ῌ -> ῃ
		return "\u1FC3"
	case '\u1FFC': // ῼ -> ῳ
		return "\u1FF3"
	default:
		return string(unicode.ToLower(r))
	}
}

// Full title case mapping: first letter gets title case, rest get lower case
func fullToTitle(r rune) string {
	switch r {
	case '\u00DF': // ß -> Ss
		return "Ss"
	case '\uFB00': // ﬀ -> Ff
		return "Ff"
	case '\uFB01': // ﬁ -> Fi
		return "Fi"
	case '\uFB02': // ﬂ -> Fl
		return "Fl"
	case '\uFB03': // ﬃ -> Ffi
		return "Ffi"
	case '\uFB04': // ﬄ -> Ffl
		return "Ffl"
	case '\uFB05': // ﬅ -> St
		return "St"
	case '\uFB06': // ﬆ -> St
		return "St"
	// Greek with ypogegrammeni -> titlecase (capital + prosgegrammeni)
	case '\u1FB3': // ᾳ -> ᾼ
		return "\u1FBC"
	case '\u1FC3': // ῃ -> ῌ
		return "\u1FCC"
	case '\u1FF3': // ῳ -> ῼ
		return "\u1FFC"
	default:
		return string(unicode.ToTitle(r))
	}
}

// convertCaseFull performs full (multi-char) case conversion for MB_CASE_UPPER/LOWER/FOLD
func convertCaseUpper(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(fullToUpper(r))
	}
	return b.String()
}

func convertCaseLower(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		b.WriteString(fullToLower(r, runes, i))
	}
	return b.String()
}

func convertCaseFold(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(fullCaseFold(r))
	}
	return b.String()
}

// convertCaseTitle performs full title case conversion.
// PHP's MB_CASE_TITLE considers a new word to start after any character that is not a letter,
// digit, or word-internal punctuation (apostrophe etc).
// It also handles Greek sigma context properly.
func convertCaseTitle(s string) string {
	runes := []rune(s)
	var b strings.Builder
	wordStart := true // next cased letter starts a new word

	for i, r := range runes {
		if wordStart && isCased(r) {
			// Title-case this character (first letter of word)
			b.WriteString(fullToTitle(r))
			wordStart = false
		} else if !wordStart && isCased(r) {
			// Lower-case (word-internal)
			b.WriteString(fullToLower(r, runes, i))
		} else if r == '\u03A3' && !wordStart {
			// Greek capital sigma in word-internal position -> context-dependent lower
			b.WriteString(fullToLower(r, runes, i))
		} else {
			b.WriteRune(r)
			// Determine if next cased character starts a new word
			if !isCaseIgnorable(r) && !isCased(r) && !unicode.IsDigit(r) && !isWordInternalPunct(r) {
				wordStart = true
			}
			// Note: case-ignorable characters, digits and word-internal punct do NOT change wordStart
		}
	}
	return b.String()
}

// convertCaseTitleSimple performs simple (single-char) title case conversion.
// Like full but uses simple mappings (no multi-char expansions), and no Greek sigma context.
func convertCaseTitleSimple(s string) string {
	runes := []rune(s)
	var b strings.Builder
	wordStart := true

	for _, r := range runes {
		if wordStart && isCased(r) {
			b.WriteRune(unicode.ToTitle(r))
			wordStart = false
		} else if !wordStart && isCased(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
			if !isCaseIgnorable(r) && !isCased(r) && !unicode.IsDigit(r) && !isWordInternalPunct(r) {
				wordStart = true
			}
		}
	}
	return b.String()
}

// fncMbConvertCase converts a string using the specified case mode.
func fncMbConvertCase(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var mode phpv.ZInt
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &mode, &enc)
	if err != nil {
		return nil, err
	}

	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_convert_case(): Argument #3 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}

	str := string(s)
	switch int(mode) {
	case mbCaseUpper: // MB_CASE_UPPER - full mapping
		return phpv.ZString(convertCaseUpper(str)).ZVal(), nil
	case mbCaseLower: // MB_CASE_LOWER - full mapping
		return phpv.ZString(convertCaseLower(str)).ZVal(), nil
	case mbCaseTitle: // MB_CASE_TITLE - full mapping
		return phpv.ZString(convertCaseTitle(str)).ZVal(), nil
	case mbCaseFold: // MB_CASE_FOLD - full mapping
		return phpv.ZString(convertCaseFold(str)).ZVal(), nil
	case mbCaseUpperSimple: // MB_CASE_UPPER_SIMPLE - simple (no multi-char)
		return phpv.ZString(strings.Map(unicode.ToUpper, str)).ZVal(), nil
	case mbCaseLowerSimple: // MB_CASE_LOWER_SIMPLE - simple (no multi-char, no sigma)
		return phpv.ZString(strings.Map(unicode.ToLower, str)).ZVal(), nil
	case mbCaseTitleSimple: // MB_CASE_TITLE_SIMPLE - simple title
		return phpv.ZString(convertCaseTitleSimple(str)).ZVal(), nil
	case mbCaseFoldSimple: // MB_CASE_FOLD_SIMPLE - same as lower simple for most cases
		return phpv.ZString(strings.Map(unicode.ToLower, str)).ZVal(), nil
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_convert_case(): Argument #2 ($mode) must be one of the MB_CASE_* constants")
	}
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

	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_strstr(): Argument #4 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
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

	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_stristr(): Argument #4 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
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

	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_ord(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
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

	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_chr(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}

	r := rune(codepoint)
	if !utf8.ValidRune(r) {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZString(string(r)).ZVal(), nil
}

// supportedEncodings lists the encodings we claim to support.
// This matches PHP's mb_list_encodings() output.
var supportedEncodings = []string{
	"ASCII",
	"UTF-8",
	"UTF-16",
	"UTF-16BE",
	"UTF-16LE",
	"UTF-32",
	"UTF-32BE",
	"UTF-32LE",
	"UTF-7",
	"UTF7-IMAP",
	"UCS-2",
	"UCS-2BE",
	"UCS-2LE",
	"UCS-4",
	"UCS-4BE",
	"UCS-4LE",
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
	"EUC-JP-2004",
	"SJIS-2004",
	"eucJP-win",
	"SJIS-mac",
	"CP51932",
	"EUC-KR",
	"UHC",
	"ISO-2022-KR",
	"Big5",
	"CP950",
	"GB18030",
	"GBK",
	"CP936",
	"EUC-CN",
	"HZ",
	"ISO-2022-JP",
	"ISO-2022-JP-2004",
	"ISO-2022-JP-MS",
	"ISO-2022-JP-KDDI",
	"CP50220",
	"CP50221",
	"CP50222",
	"JIS",
	"KOI8-R",
	"KOI8-U",
	"ArmSCII-8",
	"CP850",
	"CP866",
	"Windows-1250",
	"Windows-1251",
	"Windows-1252",
	"Windows-1253",
	"Windows-1254",
	"Windows-1255",
	"Windows-1256",
	"Windows-1257",
	"Windows-1258",
	"7bit",
	"8bit",
	"HTML-ENTITIES",
	"UUENCODE",
	"BASE64",
	"QPRINT",
}

// encodingToMime maps encoding names (uppercased) to their MIME preferred names.
var encodingToMime = map[string]string{
	"ASCII":          "US-ASCII",
	"US-ASCII":       "US-ASCII",
	"UTF-8":          "UTF-8",
	"UTF8":           "UTF-8",
	"UTF-16":         "UTF-16",
	"UTF-16BE":       "UTF-16BE",
	"UTF-16LE":       "UTF-16LE",
	"UTF-32":         "UTF-32",
	"UTF-32BE":       "UTF-32BE",
	"UTF-32LE":       "UTF-32LE",
	"UTF-7":          "UTF-7",
	"UTF7":           "UTF-7",
	"ISO-8859-1":     "ISO-8859-1",
	"ISO-8859-2":     "ISO-8859-2",
	"ISO-8859-3":     "ISO-8859-3",
	"ISO-8859-4":     "ISO-8859-4",
	"ISO-8859-5":     "ISO-8859-5",
	"ISO-8859-6":     "ISO-8859-6",
	"ISO-8859-7":     "ISO-8859-7",
	"ISO-8859-8":     "ISO-8859-8",
	"ISO-8859-9":     "ISO-8859-9",
	"ISO-8859-10":    "ISO-8859-10",
	"ISO-8859-13":    "ISO-8859-13",
	"ISO-8859-14":    "ISO-8859-14",
	"ISO-8859-15":    "ISO-8859-15",
	"ISO-8859-16":    "ISO-8859-16",
	"EUC-JP":         "EUC-JP",
	"EUCJP":          "EUC-JP",
	"SJIS":           "Shift_JIS",
	"SHIFT_JIS":      "Shift_JIS",
	"CP932":          "Shift_JIS",
	"ISO-2022-JP":    "ISO-2022-JP",
	"ISO-2022-KR":    "ISO-2022-KR",
	"EUC-KR":         "EUC-KR",
	"EUCKR":          "EUC-KR",
	"BIG5":           "Big5",
	"BIG-5":          "Big5",
	"GB18030":        "GB18030",
	"GB18030-2022":   "GB18030",
	"GBK":            "GBK",
	"HZ":             "HZ-GB-2312",
	"KOI8-R":         "KOI8-R",
	"KOI8-U":         "KOI8-U",
	"WINDOWS-1250":   "windows-1250",
	"WINDOWS-1251":   "windows-1251",
	"WINDOWS-1252":   "windows-1252",
	"WINDOWS-1253":   "windows-1253",
	"WINDOWS-1254":   "windows-1254",
	"WINDOWS-1255":   "windows-1255",
	"WINDOWS-1256":   "windows-1256",
	"WINDOWS-1257":   "windows-1257",
	"WINDOWS-1258":   "windows-1258",
	"CP1250":         "windows-1250",
	"CP1251":         "windows-1251",
	"CP1252":         "windows-1252",
	"CP1253":         "windows-1253",
	"CP1254":         "windows-1254",
	"CP1255":         "windows-1255",
	"CP1256":         "windows-1256",
	"CP1257":         "windows-1257",
	"CP1258":         "windows-1258",
}

func fncMbEncodeNumericentity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("mb_encode_numericentity() expects at least 2 arguments, %d given", len(args)))
	}
	if len(args) > 2 && args[2] != nil && args[2].GetType() == phpv.ZtString {
		encStr := args[2].String()
		if !isValidEncoding(encStr) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_encode_numericentity(): Argument #3 ($encoding) must be a valid encoding, \"%s\" given", encStr))
		}
	}
	str := args[0].String()
	convmap := args[1]
	if convmap.GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_encode_numericentity(): Argument #2 ($map) must be of type array")
	}
	mapArr := convmap.Value().(*phpv.ZArray)
	var mapVals []int
	for _, v := range mapArr.Iterate(ctx) {
		mapVals = append(mapVals, int(v.AsInt(ctx)))
	}
	isHex := false
	if len(args) > 3 && args[3] != nil {
		isHex = args[3].AsBool(ctx) == phpv.ZTrue
	}
	var result strings.Builder
	for _, r := range str {
		encoded := false
		for i := 0; i+3 < len(mapVals); i += 4 {
			if int(r) >= mapVals[i] && int(r) <= mapVals[i+1] {
				cp := (int(r) + mapVals[i+2]) & mapVals[i+3]
				if isHex {
					result.WriteString(fmt.Sprintf("&#x%X;", cp))
				} else {
					result.WriteString(fmt.Sprintf("&#%d;", cp))
				}
				encoded = true
				break
			}
		}
		if !encoded {
			result.WriteRune(r)
		}
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func fncMbDecodeNumericentity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("mb_decode_numericentity() expects at least 2 arguments, %d given", len(args)))
	}
	if len(args) > 2 && args[2] != nil && args[2].GetType() == phpv.ZtString {
		encStr := args[2].String()
		if !isValidEncoding(encStr) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_decode_numericentity(): Argument #3 ($encoding) must be a valid encoding, \"%s\" given", encStr))
		}
	}
	str := args[0].String()
	convmap := args[1]
	if convmap.GetType() != phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_decode_numericentity(): Argument #2 ($map) must be of type array")
	}
	mapArr := convmap.Value().(*phpv.ZArray)
	var mapVals []int
	for _, v := range mapArr.Iterate(ctx) {
		mapVals = append(mapVals, int(v.AsInt(ctx)))
	}
	var result strings.Builder
	i := 0
	for i < len(str) {
		if str[i] == '&' && i+2 < len(str) && str[i+1] == '#' {
			j := i + 2
			isHex := false
			if j < len(str) && (str[j] == 'x' || str[j] == 'X') {
				isHex = true
				j++
			}
			numStart := j
			for j < len(str) && ((str[j] >= '0' && str[j] <= '9') || (isHex && ((str[j] >= 'a' && str[j] <= 'f') || (str[j] >= 'A' && str[j] <= 'F')))) {
				j++
			}
			if j > numStart {
				var cp int
				if isHex {
					fmt.Sscanf(str[numStart:j], "%x", &cp)
				} else {
					fmt.Sscanf(str[numStart:j], "%d", &cp)
				}
				decoded := false
				for k := 0; k+3 < len(mapVals); k += 4 {
					orig := (cp & mapVals[k+3]) - mapVals[k+2]
					if orig >= mapVals[k] && orig <= mapVals[k+1] {
						result.WriteRune(rune(orig))
						decoded = true
						break
					}
				}
				if !decoded {
					endPos := j
					if endPos < len(str) && str[endPos] == ';' {
						endPos++
					}
					result.WriteString(str[i:endPos])
				} else if j < len(str) && str[j] == ';' {
					j++ // consume trailing semicolon
				}
				i = j
				continue
			}
		}
		result.WriteByte(str[i])
		i++
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func fncMbDecodeMimeheader(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	str := string(s)
	var result strings.Builder
	i := 0
	for i < len(str) {
		if i+2 < len(str) && str[i] == '=' && str[i+1] == '?' {
			end := strings.LastIndex(str[i+2:], "?=")
			if end >= 0 {
				parts := strings.SplitN(str[i+2:i+2+end], "?", 3)
				if len(parts) == 3 {
					charset, encType, text := parts[0], strings.ToUpper(parts[1]), parts[2]
					var decoded string
					if encType == "B" {
						decoded = mimeDecBase64(text)
					} else if encType == "Q" {
						decoded = mimeDecQP(text)
					} else {
						decoded = text
					}
					if n := normalizeEncodingName(charset); n != "UTF-8" && n != "UTF8" {
						c, _, _ := decodeToUTF8([]byte(decoded), n)
						result.Write(c)
					} else {
						result.WriteString(decoded)
					}
					i = i + 2 + end + 2
					continue
				}
			}
		}
		result.WriteByte(str[i])
		i++
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func mimeDecBase64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	var buf uint32
	var bits int
	for _, c := range s {
		if c == '=' {
			break
		}
		idx := strings.IndexRune(chars, c)
		if idx < 0 {
			continue
		}
		buf = (buf << 6) | uint32(idx)
		bits += 6
		if bits >= 8 {
			bits -= 8
			result = append(result, byte(buf>>uint(bits)))
			buf &= (1 << uint(bits)) - 1
		}
	}
	return string(result)
}

func mimeDecQP(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '=' && i+2 < len(s) {
			var b byte
			valid := true
			for _, c := range s[i+1 : i+3] {
				b <<= 4
				if c >= '0' && c <= '9' {
					b |= byte(c - '0')
				} else if c >= 'a' && c <= 'f' {
					b |= byte(c - 'a' + 10)
				} else if c >= 'A' && c <= 'F' {
					b |= byte(c - 'A' + 10)
				} else {
					valid = false
					break
				}
			}
			if valid {
				result = append(result, b)
				i += 2
				continue
			}
		}
		if s[i] == '_' {
			result = append(result, ' ')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func fncMbEncodeMimeheader(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var charset, transferEnc, linefeed *phpv.ZString
	var indent *phpv.ZInt
	_, err := core.Expand(ctx, args, &s, &charset, &transferEnc, &linefeed, &indent)
	if err != nil {
		return nil, err
	}
	str := string(s)
	cs := "UTF-8"
	if charset != nil {
		cs = string(*charset)
	}
	needsEnc := false
	for i := 0; i < len(str); i++ {
		if str[i] > 127 {
			needsEnc = true
			break
		}
	}
	if !needsEnc {
		return phpv.ZString(str).ZVal(), nil
	}
	enc := "B"
	if transferEnc != nil {
		enc = strings.ToUpper(string(*transferEnc))
	}
	var encoded string
	if enc == "Q" {
		var r strings.Builder
		for i := 0; i < len(str); i++ {
			if s[i] == ' ' {
				r.WriteByte('_')
			} else if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= '0' && s[i] <= '9') {
				r.WriteByte(s[i])
			} else {
				r.WriteString(fmt.Sprintf("=%02X", s[i]))
			}
		}
		encoded = r.String()
	} else {
		const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		data := []byte(str)
		var r strings.Builder
		for i := 0; i < len(data); i += 3 {
			var b0, b1, b2 byte
			b0 = data[i]
			if i+1 < len(data) {
				b1 = data[i+1]
			}
			if i+2 < len(data) {
				b2 = data[i+2]
			}
			r.WriteByte(b64[b0>>2])
			r.WriteByte(b64[((b0&3)<<4)|(b1>>4)])
			if i+1 < len(data) {
				r.WriteByte(b64[((b1&0xF)<<2)|(b2>>6)])
			} else {
				r.WriteByte('=')
			}
			if i+2 < len(data) {
				r.WriteByte(b64[b2&0x3F])
			} else {
				r.WriteByte('=')
			}
		}
		encoded = r.String()
	}
	return phpv.ZString(fmt.Sprintf("=?%s?%s?%s?=", cs, enc, encoded)).ZVal(), nil
}

func fncMbConvertKana(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var option, enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &option, &enc)
	if err != nil {
		return nil, err
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_convert_kana(): Argument #3 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}

	optStr := "KV"
	if option != nil {
		optStr = string(*option)
	}

	// Parse option flags
	var (
		optAsciiToZenkakuAlpha   bool // 'A' ASCII alpha -> full-width
		optZenkakuAlphaToAscii   bool // 'a' full-width alpha -> ASCII
		optAsciiToZenkakuNum     bool // 'N' ASCII digits -> full-width
		optZenkakuNumToAscii     bool // 'n' full-width digits -> ASCII
		optAsciiToZenkakuSpace   bool // 'S' ASCII space -> full-width
		optZenkakuSpaceToAscii   bool // 's' full-width space -> ASCII
		optHiraganaToKatakana    bool // 'C' hiragana -> katakana
		optKatakanaToHiragana    bool // 'c' katakana -> hiragana
		optHankakuKataToZenkaku  bool // 'K' half-width katakana -> full-width katakana
		optZenkakuKataToHankaku  bool // 'k' full-width katakana -> half-width katakana
		optHankakuKataToHiragana bool // 'H' half-width katakana -> hiragana
		optHiraganaToHankakuKata bool // 'h' hiragana -> half-width katakana
		optCombineDakuten        bool // 'V' combine voiced sound marks (for K and H)
		optAsciiToZenkakuAlnum   bool // 'R' ASCII alpha+num -> full-width (like A+N)
		optZenkakuAlnumToAscii   bool // 'r' full-width alpha+num -> ASCII (like a+n)
	)

	for _, c := range optStr {
		switch c {
		case 'A':
			optAsciiToZenkakuAlpha = true
		case 'a':
			optZenkakuAlphaToAscii = true
		case 'N':
			optAsciiToZenkakuNum = true
		case 'n':
			optZenkakuNumToAscii = true
		case 'S':
			optAsciiToZenkakuSpace = true
		case 's':
			optZenkakuSpaceToAscii = true
		case 'C':
			optHiraganaToKatakana = true
		case 'c':
			optKatakanaToHiragana = true
		case 'K':
			optHankakuKataToZenkaku = true
		case 'k':
			optZenkakuKataToHankaku = true
		case 'H':
			optHankakuKataToHiragana = true
		case 'h':
			optHiraganaToHankakuKata = true
		case 'V':
			optCombineDakuten = true
		case 'R':
			optAsciiToZenkakuAlnum = true
		case 'r':
			optZenkakuAlnumToAscii = true
		}
	}

	_ = optCombineDakuten
	_ = optHankakuKataToZenkaku
	_ = optZenkakuKataToHankaku
	_ = optHankakuKataToHiragana
	_ = optHiraganaToHankakuKata
	_ = optAsciiToZenkakuAlnum
	_ = optZenkakuAlnumToAscii

	runes := []rune(string(s))
	result := make([]rune, 0, len(runes))

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// C: Hiragana -> Katakana
		// U+3041-U+3096 main hiragana -> U+30A1-U+30F6
		// U+309D-U+309E iteration marks -> U+30FD-U+30FE
		if optHiraganaToKatakana {
			if r >= 0x3041 && r <= 0x3096 {
				result = append(result, r+0x60)
				continue
			}
			if r == 0x309D || r == 0x309E {
				result = append(result, r+0x60)
				continue
			}
		}

		// c: Katakana -> Hiragana
		// U+30A1-U+30F6 main katakana -> U+3041-U+3096
		// U+30FD-U+30FE iteration marks -> U+309D-U+309E
		if optKatakanaToHiragana {
			if r >= 0x30A1 && r <= 0x30F6 {
				result = append(result, r-0x60)
				continue
			}
			if r == 0x30FD || r == 0x30FE {
				result = append(result, r-0x60)
				continue
			}
		}

		// A: ASCII alphabetic -> full-width
		if (optAsciiToZenkakuAlpha || optAsciiToZenkakuAlnum) &&
			((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			result = append(result, r-0x20+0xFF00)
			continue
		}

		// a: Full-width alphabetic -> ASCII
		if (optZenkakuAlphaToAscii || optZenkakuAlnumToAscii) &&
			((r >= 0xFF21 && r <= 0xFF3A) || (r >= 0xFF41 && r <= 0xFF5A)) {
			result = append(result, r-0xFF00+0x20)
			continue
		}

		// N: ASCII digits -> full-width
		if (optAsciiToZenkakuNum || optAsciiToZenkakuAlnum) && r >= '0' && r <= '9' {
			result = append(result, r-0x20+0xFF00)
			continue
		}

		// n: Full-width digits -> ASCII
		if (optZenkakuNumToAscii || optZenkakuAlnumToAscii) && r >= 0xFF10 && r <= 0xFF19 {
			result = append(result, r-0xFF00+0x20)
			continue
		}

		// S: ASCII space -> full-width space
		if optAsciiToZenkakuSpace && r == ' ' {
			result = append(result, 0x3000)
			continue
		}

		// s: Full-width space -> ASCII space
		if optZenkakuSpaceToAscii && r == 0x3000 {
			result = append(result, ' ')
			continue
		}

		// K: Half-width katakana -> full-width katakana
		if optHankakuKataToZenkaku && r >= 0xFF65 && r <= 0xFF9F {
			fullwidth := halfToFullKatakana(r)
			if optCombineDakuten && i+1 < len(runes) {
				combined := combineDakuten(fullwidth, runes[i+1])
				if combined != 0 {
					result = append(result, combined)
					i++
					continue
				}
			}
			result = append(result, fullwidth)
			continue
		}

		// H: Half-width katakana -> hiragana
		if optHankakuKataToHiragana && r >= 0xFF65 && r <= 0xFF9F {
			fullwidth := halfToFullKatakana(r)
			// Convert katakana to hiragana
			if fullwidth >= 0x30A1 && fullwidth <= 0x30F6 {
				hiragana := fullwidth - 0x60
				if optCombineDakuten && i+1 < len(runes) {
					combined := combineDakuten(fullwidth, runes[i+1])
					if combined != 0 {
						if combined >= 0x30A1 && combined <= 0x30F6 {
							result = append(result, combined-0x60)
						} else {
							result = append(result, combined)
						}
						i++
						continue
					}
				}
				result = append(result, hiragana)
			} else {
				result = append(result, fullwidth)
			}
			continue
		}

		// k: Full-width katakana -> half-width katakana
		if optZenkakuKataToHankaku && r >= 0x30A1 && r <= 0x30F6 {
			hw, dakuten := fullToHalfKatakana(r)
			if hw != 0 {
				result = append(result, hw)
				if dakuten != 0 {
					result = append(result, dakuten)
				}
				continue
			}
		}

		// h: Hiragana -> half-width katakana
		if optHiraganaToHankakuKata && r >= 0x3041 && r <= 0x3096 {
			katakana := r + 0x60
			hw, dakuten := fullToHalfKatakana(katakana)
			if hw != 0 {
				result = append(result, hw)
				if dakuten != 0 {
					result = append(result, dakuten)
				}
				continue
			}
		}

		result = append(result, r)
	}

	return phpv.ZString(string(result)).ZVal(), nil
}

// halfToFullKatakana converts a half-width katakana character to full-width.
func halfToFullKatakana(r rune) rune {
	// Half-width katakana: U+FF65 - U+FF9F
	halfToFull := map[rune]rune{
		0xFF66: 0x30F2, // ヲ
		0xFF67: 0x30A1, // ァ
		0xFF68: 0x30A3, // ィ
		0xFF69: 0x30A5, // ゥ
		0xFF6A: 0x30A7, // ェ
		0xFF6B: 0x30A9, // ォ
		0xFF6C: 0x30E3, // ャ
		0xFF6D: 0x30E5, // ュ
		0xFF6E: 0x30E7, // ョ
		0xFF6F: 0x30C3, // ッ
		0xFF70: 0x30FC, // ー
		0xFF71: 0x30A2, // ア
		0xFF72: 0x30A4, // イ
		0xFF73: 0x30A6, // ウ
		0xFF74: 0x30A8, // エ
		0xFF75: 0x30AA, // オ
		0xFF76: 0x30AB, // カ
		0xFF77: 0x30AD, // キ
		0xFF78: 0x30AF, // ク
		0xFF79: 0x30B1, // ケ
		0xFF7A: 0x30B3, // コ
		0xFF7B: 0x30B5, // サ
		0xFF7C: 0x30B7, // シ
		0xFF7D: 0x30B9, // ス
		0xFF7E: 0x30BB, // セ
		0xFF7F: 0x30BD, // ソ
		0xFF80: 0x30BF, // タ
		0xFF81: 0x30C1, // チ
		0xFF82: 0x30C4, // ツ
		0xFF83: 0x30C6, // テ
		0xFF84: 0x30C8, // ト
		0xFF85: 0x30CA, // ナ
		0xFF86: 0x30CB, // ニ
		0xFF87: 0x30CC, // ヌ
		0xFF88: 0x30CD, // ネ
		0xFF89: 0x30CE, // ノ
		0xFF8A: 0x30CF, // ハ
		0xFF8B: 0x30D2, // ヒ
		0xFF8C: 0x30D5, // フ
		0xFF8D: 0x30D8, // ヘ
		0xFF8E: 0x30DB, // ホ
		0xFF8F: 0x30DE, // マ
		0xFF90: 0x30DF, // ミ
		0xFF91: 0x30E0, // ム
		0xFF92: 0x30E1, // メ
		0xFF93: 0x30E2, // モ
		0xFF94: 0x30E4, // ヤ
		0xFF95: 0x30E6, // ユ
		0xFF96: 0x30E8, // ヨ
		0xFF97: 0x30E9, // ラ
		0xFF98: 0x30EA, // リ
		0xFF99: 0x30EB, // ル
		0xFF9A: 0x30EC, // レ
		0xFF9B: 0x30ED, // ロ
		0xFF9C: 0x30EF, // ワ
		0xFF9D: 0x30F3, // ン
		0xFF9E: 0x309B, // ゛ (voiced mark - not really katakana)
		0xFF9F: 0x309C, // ゜ (semi-voiced mark)
		0xFF65: 0x30FB, // ・
	}
	if full, ok := halfToFull[r]; ok {
		return full
	}
	return r
}

// combineDakuten tries to combine a katakana character with a following voiced/semi-voiced mark.
func combineDakuten(base rune, next rune) rune {
	if next == 0xFF9E || next == 0x309B { // voiced sound mark (half-width or full-width)
		// Check if base can take dakuten
		switch base {
		case 0x30AB: return 0x30AC // カ->ガ
		case 0x30AD: return 0x30AE // キ->ギ
		case 0x30AF: return 0x30B0 // ク->グ
		case 0x30B1: return 0x30B2 // ケ->ゲ
		case 0x30B3: return 0x30B4 // コ->ゴ
		case 0x30B5: return 0x30B6 // サ->ザ
		case 0x30B7: return 0x30B8 // シ->ジ
		case 0x30B9: return 0x30BA // ス->ズ
		case 0x30BB: return 0x30BC // セ->ゼ
		case 0x30BD: return 0x30BE // ソ->ゾ
		case 0x30BF: return 0x30C0 // タ->ダ
		case 0x30C1: return 0x30C2 // チ->ヂ
		case 0x30C4: return 0x30C5 // ツ->ヅ
		case 0x30C6: return 0x30C7 // テ->デ
		case 0x30C8: return 0x30C9 // ト->ド
		case 0x30CF: return 0x30D0 // ハ->バ
		case 0x30D2: return 0x30D3 // ヒ->ビ
		case 0x30D5: return 0x30D6 // フ->ブ
		case 0x30D8: return 0x30D9 // ヘ->ベ
		case 0x30DB: return 0x30DC // ホ->ボ
		case 0x30A6: return 0x30F4 // ウ->ヴ
		}
	}
	if next == 0xFF9F || next == 0x309C { // semi-voiced sound mark
		switch base {
		case 0x30CF: return 0x30D1 // ハ->パ
		case 0x30D2: return 0x30D4 // ヒ->ピ
		case 0x30D5: return 0x30D7 // フ->プ
		case 0x30D8: return 0x30DA // ヘ->ペ
		case 0x30DB: return 0x30DD // ホ->ポ
		}
	}
	return 0
}

// fullToHalfKatakana converts a full-width katakana character to half-width.
// Returns the half-width character and an optional dakuten/handakuten mark.
func fullToHalfKatakana(r rune) (rune, rune) {
	// Check for dakuten characters first
	switch r {
	case 0x30AC: return 0xFF76, 0xFF9E // ガ
	case 0x30AE: return 0xFF77, 0xFF9E // ギ
	case 0x30B0: return 0xFF78, 0xFF9E // グ
	case 0x30B2: return 0xFF79, 0xFF9E // ゲ
	case 0x30B4: return 0xFF7A, 0xFF9E // ゴ
	case 0x30B6: return 0xFF7B, 0xFF9E // ザ
	case 0x30B8: return 0xFF7C, 0xFF9E // ジ
	case 0x30BA: return 0xFF7D, 0xFF9E // ズ
	case 0x30BC: return 0xFF7E, 0xFF9E // ゼ
	case 0x30BE: return 0xFF7F, 0xFF9E // ゾ
	case 0x30C0: return 0xFF80, 0xFF9E // ダ
	case 0x30C2: return 0xFF81, 0xFF9E // ヂ
	case 0x30C5: return 0xFF82, 0xFF9E // ヅ
	case 0x30C7: return 0xFF83, 0xFF9E // デ
	case 0x30C9: return 0xFF84, 0xFF9E // ド
	case 0x30D0: return 0xFF8A, 0xFF9E // バ
	case 0x30D3: return 0xFF8B, 0xFF9E // ビ
	case 0x30D6: return 0xFF8C, 0xFF9E // ブ
	case 0x30D9: return 0xFF8D, 0xFF9E // ベ
	case 0x30DC: return 0xFF8E, 0xFF9E // ボ
	case 0x30F4: return 0xFF73, 0xFF9E // ヴ
	// Handakuten
	case 0x30D1: return 0xFF8A, 0xFF9F // パ
	case 0x30D4: return 0xFF8B, 0xFF9F // ピ
	case 0x30D7: return 0xFF8C, 0xFF9F // プ
	case 0x30DA: return 0xFF8D, 0xFF9F // ペ
	case 0x30DD: return 0xFF8E, 0xFF9F // ポ
	}

	// Simple mapping (reverse of halfToFullKatakana)
	fullToHalf := map[rune]rune{
		0x30F2: 0xFF66, // ヲ
		0x30A1: 0xFF67, // ァ
		0x30A3: 0xFF68, // ィ
		0x30A5: 0xFF69, // ゥ
		0x30A7: 0xFF6A, // ェ
		0x30A9: 0xFF6B, // ォ
		0x30E3: 0xFF6C, // ャ
		0x30E5: 0xFF6D, // ュ
		0x30E7: 0xFF6E, // ョ
		0x30C3: 0xFF6F, // ッ
		0x30FC: 0xFF70, // ー
		0x30A2: 0xFF71, // ア
		0x30A4: 0xFF72, // イ
		0x30A6: 0xFF73, // ウ
		0x30A8: 0xFF74, // エ
		0x30AA: 0xFF75, // オ
		0x30AB: 0xFF76, // カ
		0x30AD: 0xFF77, // キ
		0x30AF: 0xFF78, // ク
		0x30B1: 0xFF79, // ケ
		0x30B3: 0xFF7A, // コ
		0x30B5: 0xFF7B, // サ
		0x30B7: 0xFF7C, // シ
		0x30B9: 0xFF7D, // ス
		0x30BB: 0xFF7E, // セ
		0x30BD: 0xFF7F, // ソ
		0x30BF: 0xFF80, // タ
		0x30C1: 0xFF81, // チ
		0x30C4: 0xFF82, // ツ
		0x30C6: 0xFF83, // テ
		0x30C8: 0xFF84, // ト
		0x30CA: 0xFF85, // ナ
		0x30CB: 0xFF86, // ニ
		0x30CC: 0xFF87, // ヌ
		0x30CD: 0xFF88, // ネ
		0x30CE: 0xFF89, // ノ
		0x30CF: 0xFF8A, // ハ
		0x30D2: 0xFF8B, // ヒ
		0x30D5: 0xFF8C, // フ
		0x30D8: 0xFF8D, // ヘ
		0x30DB: 0xFF8E, // ホ
		0x30DE: 0xFF8F, // マ
		0x30DF: 0xFF90, // ミ
		0x30E0: 0xFF91, // ム
		0x30E1: 0xFF92, // メ
		0x30E2: 0xFF93, // モ
		0x30E4: 0xFF94, // ヤ
		0x30E6: 0xFF95, // ユ
		0x30E8: 0xFF96, // ヨ
		0x30E9: 0xFF97, // ラ
		0x30EA: 0xFF98, // リ
		0x30EB: 0xFF99, // ル
		0x30EC: 0xFF9A, // レ
		0x30ED: 0xFF9B, // ロ
		0x30EF: 0xFF9C, // ワ
		0x30F3: 0xFF9D, // ン
		0x30FB: 0xFF65, // ・
	}
	if hw, ok := fullToHalf[r]; ok {
		return hw, 0
	}
	return r, 0
}

func fncMbRegexSetOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZString("msr").ZVal(), nil
	}
	return phpv.ZString(args[0].String()).ZVal(), nil
}

func fncMbUcfirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}
	str := string(s)
	if len(str) == 0 {
		return s.ZVal(), nil
	}
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return phpv.ZString(string(runes)).ZVal(), nil
}

func fncMbLcfirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}
	str := string(s)
	if len(str) == 0 {
		return s.ZVal(), nil
	}
	runes := []rune(str)
	runes[0] = unicode.ToLower(runes[0])
	return phpv.ZString(string(runes)).ZVal(), nil
}

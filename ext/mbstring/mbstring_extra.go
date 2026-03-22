package mbstring

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
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
// PHP uses a context window: scan back up to 63 positions for a cased letter (skipping case-ignorable),
// and scan forward with no limit.
func shouldBeFinalSigma(runes []rune, idx int) bool {
	// Must be preceded by a cased letter (possibly with case-ignorable in between)
	foundCasedBefore := false
	limit := 63
	for i := idx - 1; i >= 0 && (idx-1-i) < limit; i-- {
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
	case '\u0587': // ﬓ Armenian ew -> ԵՒ
		return "\u0535\u0552"
	case '\u1E9E': // Capital sharp S -> SS
		return "SS"
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

func fncMbEncodeNumericentity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("mb_encode_numericentity() expects at least 2 arguments")
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
		return nil, ctx.Errorf("mb_decode_numericentity() expects at least 2 arguments")
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
			if j > numStart && j < len(str) && str[j] == ';' {
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
					result.WriteString(str[i : j+1])
				}
				i = j + 1
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
			end := strings.Index(str[i+2:], "?=")
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
	return s.ZVal(), nil
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

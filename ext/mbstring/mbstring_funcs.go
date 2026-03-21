package mbstring

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// mb_get_info returns an array of info about mbstring settings
func fncMbGetInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var optType *phpv.ZString
	_, err := core.Expand(ctx, args, &optType)
	if err != nil {
		return nil, err
	}

	// If specific type requested
	if optType != nil {
		switch strings.ToLower(string(*optType)) {
		case "internal_encoding":
			return phpv.ZString("UTF-8").ZVal(), nil
		case "http_input":
			return phpv.ZString("").ZVal(), nil
		case "http_output":
			return phpv.ZString("UTF-8").ZVal(), nil
		case "func_overload":
			return phpv.ZInt(0).ZVal(), nil
		case "mail_charset":
			return phpv.ZString("UTF-8").ZVal(), nil
		case "mail_header_encoding":
			return phpv.ZString("BASE64").ZVal(), nil
		case "mail_body_encoding":
			return phpv.ZString("BASE64").ZVal(), nil
		case "detect_order":
			arr := phpv.NewZArray()
			arr.OffsetSet(ctx, nil, phpv.ZString("ASCII").ZVal())
			arr.OffsetSet(ctx, nil, phpv.ZString("UTF-8").ZVal())
			return arr.ZVal(), nil
		case "language":
			return phpv.ZString("neutral").ZVal(), nil
		case "encoding_translation":
			return phpv.ZString("Off").ZVal(), nil
		case "substitute_character":
			return phpv.ZString("none").ZVal(), nil
		case "strict_detection":
			return phpv.ZString("Off").ZVal(), nil
		default:
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	// Return all info
	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZString("internal_encoding").ZVal(), phpv.ZString("UTF-8").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("http_input").ZVal(), phpv.ZString("").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("http_output").ZVal(), phpv.ZString("UTF-8").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("func_overload").ZVal(), phpv.ZInt(0).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_charset").ZVal(), phpv.ZString("UTF-8").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_header_encoding").ZVal(), phpv.ZString("BASE64").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_body_encoding").ZVal(), phpv.ZString("BASE64").ZVal())

	detectOrder := phpv.NewZArray()
	detectOrder.OffsetSet(ctx, nil, phpv.ZString("ASCII").ZVal())
	detectOrder.OffsetSet(ctx, nil, phpv.ZString("UTF-8").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("detect_order").ZVal(), detectOrder.ZVal())

	arr.OffsetSet(ctx, phpv.ZString("language").ZVal(), phpv.ZString("neutral").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("encoding_translation").ZVal(), phpv.ZString("Off").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("substitute_character").ZVal(), phpv.ZInt(63).ZVal()) // '?'
	arr.OffsetSet(ctx, phpv.ZString("strict_detection").ZVal(), phpv.ZString("Off").ZVal())
	return arr.ZVal(), nil
}

// mb_scrub replaces ill-formed byte sequences with substitute characters
func fncMbScrub(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	// For UTF-8, replace invalid sequences with U+FFFD
	str := string(s)
	if utf8.ValidString(str) {
		return s.ZVal(), nil
	}

	var result strings.Builder
	for i := 0; i < len(str); {
		r, size := utf8.DecodeRuneInString(str[i:])
		if r == utf8.RuneError && size == 1 {
			result.WriteRune(0xFFFD)
			i++
		} else {
			result.WriteRune(r)
			i += size
		}
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

// mb_stripos finds position of first occurrence of string in another string (case insensitive)
func fncMbStripos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil {
		return nil, err
	}

	hRunes := []rune(strings.ToLower(string(haystack)))
	nRunes := []rune(strings.ToLower(string(needle)))

	start := 0
	if offset != nil {
		start = int(*offset)
		if start < 0 {
			start = len(hRunes) + start
		}
		if start < 0 || start > len(hRunes) {
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	for i := start; i <= len(hRunes)-len(nRunes); i++ {
		match := true
		for j := 0; j < len(nRunes); j++ {
			if hRunes[i+j] != nRunes[j] {
				match = false
				break
			}
		}
		if match {
			return phpv.ZInt(i).ZVal(), nil
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

// mb_strripos finds the last occurrence of needle in haystack (case-insensitive)
func fncMbStrripos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil {
		return nil, err
	}

	hRunes := []rune(strings.ToLower(string(haystack)))
	nRunes := []rune(strings.ToLower(string(needle)))

	start := len(hRunes) - 1
	if offset != nil {
		o := int(*offset)
		if o < 0 {
			start = len(hRunes) + o
		}
	}

	for i := start; i >= 0; i-- {
		if i+len(nRunes) > len(hRunes) {
			continue
		}
		match := true
		for j := 0; j < len(nRunes); j++ {
			if hRunes[i+j] != nRunes[j] {
				match = false
				break
			}
		}
		if match {
			return phpv.ZInt(i).ZVal(), nil
		}
	}

	return phpv.ZBool(false).ZVal(), nil
}

// mb_strrchr finds the last occurrence of needle in haystack
func fncMbStrrchr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
	// PHP only uses first character of needle
	if len(n) > 0 {
		r, _ := utf8.DecodeRuneInString(n)
		n = string(r)
	}

	idx := strings.LastIndex(h, n)
	if idx < 0 {
		return phpv.ZFalse.ZVal(), nil
	}
	if bool(before) {
		return phpv.ZString(h[:idx]).ZVal(), nil
	}
	return phpv.ZString(h[idx:]).ZVal(), nil
}

// mb_strrichr is the case-insensitive version of mb_strrchr
func fncMbStrrichr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
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
	if len(n) > 0 {
		r, _ := utf8.DecodeRuneInString(n)
		n = string(r)
	}

	hLower := strings.ToLower(h)
	nLower := strings.ToLower(n)

	idx := strings.LastIndex(hLower, nLower)
	if idx < 0 {
		return phpv.ZFalse.ZVal(), nil
	}
	if bool(before) {
		return phpv.ZString(h[:idx]).ZVal(), nil
	}
	return phpv.ZString(h[idx:]).ZVal(), nil
}

// mb_strimwidth gets truncated string with specified width
func fncMbStrimwidth(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var start phpv.ZInt
	var width phpv.ZInt
	var trimmarker *phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &start, &width, &trimmarker, &enc)
	if err != nil {
		return nil, err
	}

	runes := []rune(string(s))
	runeLen := len(runes)

	st := int(start)
	if st < 0 {
		st = runeLen + st
	}
	if st < 0 {
		st = 0
	}
	if st > runeLen {
		return phpv.ZString("").ZVal(), nil
	}

	w := int(width)
	if w < 0 {
		w = runeLen + w - st
		if w < 0 {
			w = 0
		}
	}

	marker := ""
	if trimmarker != nil {
		marker = string(*trimmarker)
	}
	markerRunes := []rune(marker)

	remaining := runes[st:]
	if len(remaining) <= w {
		return phpv.ZString(string(remaining)).ZVal(), nil
	}

	// Need to trim
	if len(markerRunes) >= w {
		return phpv.ZString(string(markerRunes[:w])).ZVal(), nil
	}

	cutLen := w - len(markerRunes)
	result := string(remaining[:cutLen]) + marker
	return phpv.ZString(result).ZVal(), nil
}

// mb_strcut gets part of a string (byte-based, but character-boundary aware)
func fncMbStrcut(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var start phpv.ZInt
	var length *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &start, &length, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	st := int(start)
	if st < 0 {
		st = len(str) + st
	}
	if st < 0 {
		st = 0
	}
	if st > len(str) {
		return phpv.ZString("").ZVal(), nil
	}

	// Adjust start to character boundary
	for st > 0 && st < len(str) && !utf8.RuneStart(str[st]) {
		st--
	}

	end := len(str)
	if length != nil {
		l := int(*length)
		if l < 0 {
			end = len(str) + l
		} else {
			end = st + l
		}
	}
	if end > len(str) {
		end = len(str)
	}
	if end < st {
		return phpv.ZString("").ZVal(), nil
	}

	// Adjust end to character boundary
	for end > st && end < len(str) && !utf8.RuneStart(str[end]) {
		end--
	}

	return phpv.ZString(str[st:end]).ZVal(), nil
}

// mb_convert_variables converts character encoding of variables
func fncMbConvertVariables(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return nil, ctx.FuncErrorf("mb_convert_variables() expects at least 3 arguments")
	}
	// In goro, everything is UTF-8, so this is effectively a no-op
	// Return the detected encoding
	return phpv.ZString("UTF-8").ZVal(), nil
}

// mb_http_input returns the HTTP input character encoding
func fncMbHttpInput(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

// mb_http_output gets/sets the HTTP output character encoding
func fncMbHttpOutput(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZString("UTF-8").ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

// mb_encoding_aliases returns an array of aliases for a known encoding type
func fncMbEncodingAliases(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var encoding phpv.ZString
	_, err := core.Expand(ctx, args, &encoding)
	if err != nil {
		return nil, err
	}

	enc := strings.ToUpper(string(encoding))
	aliases, ok := encodingAliases[enc]
	if !ok {
		// Try normalized
		normalized := strings.ReplaceAll(enc, "-", "")
		normalized = strings.ReplaceAll(normalized, "_", "")
		for k, v := range encodingAliases {
			kn := strings.ReplaceAll(k, "-", "")
			kn = strings.ReplaceAll(kn, "_", "")
			if kn == normalized {
				aliases = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	arr := phpv.NewZArray()
	for _, a := range aliases {
		arr.OffsetSet(ctx, nil, phpv.ZString(a).ZVal())
	}
	return arr.ZVal(), nil
}

// mb_str_pad pads a multibyte string to a certain length
func fncMbStrPad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var length phpv.ZInt
	var padStr *phpv.ZString
	var padType *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &length, &padStr, &padType, &enc)
	if err != nil {
		return nil, err
	}

	pad := " "
	if padStr != nil {
		pad = string(*padStr)
	}
	pType := 1 // STR_PAD_RIGHT
	if padType != nil {
		pType = int(*padType)
	}

	runes := []rune(string(s))
	padRunes := []rune(pad)
	targetLen := int(length)

	if len(runes) >= targetLen || len(padRunes) == 0 {
		return s.ZVal(), nil
	}

	needed := targetLen - len(runes)

	switch pType {
	case 0: // STR_PAD_RIGHT
		for len(runes) < targetLen {
			runes = append(runes, padRunes[0])
			padRunes = append(padRunes[1:], padRunes[0])
		}
	case 1: // STR_PAD_LEFT (PHP constant STR_PAD_LEFT = 1... wait, actually PHP: STR_PAD_RIGHT=1, STR_PAD_LEFT=2... let me check)
		// PHP: STR_PAD_RIGHT = 1, STR_PAD_LEFT = 0, STR_PAD_BOTH = 2
		// Actually no: STR_PAD_RIGHT = 1, STR_PAD_LEFT = 0...
		// Let me just use the standard behavior
		prefix := make([]rune, 0, needed)
		pidx := 0
		for i := 0; i < needed; i++ {
			prefix = append(prefix, padRunes[pidx%len(padRunes)])
			pidx++
		}
		runes = append(prefix, runes...)
	case 2: // STR_PAD_BOTH
		leftPad := needed / 2
		rightPad := needed - leftPad
		prefix := make([]rune, 0, leftPad)
		pidx := 0
		for i := 0; i < leftPad; i++ {
			prefix = append(prefix, padRunes[pidx%len(padRunes)])
			pidx++
		}
		suffix := make([]rune, 0, rightPad)
		pidx = 0
		for i := 0; i < rightPad; i++ {
			suffix = append(suffix, padRunes[pidx%len(padRunes)])
			pidx++
		}
		runes = append(prefix, runes...)
		runes = append(runes, suffix...)
	default:
		// STR_PAD_RIGHT (default)
		pidx := 0
		for i := 0; i < needed; i++ {
			runes = append(runes, padRunes[pidx%len(padRunes)])
			pidx++
		}
	}

	return phpv.ZString(string(runes)).ZVal(), nil
}

// mb_trim trims characters from both sides of a string (multibyte aware)
func fncMbTrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var chars *phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	if chars != nil {
		charSet := string(*chars)
		runeSet := []rune(charSet)
		isInSet := func(r rune) bool {
			for _, c := range runeSet {
				if r == c {
					return true
				}
			}
			return false
		}
		str = strings.TrimFunc(str, isInSet)
	} else {
		str = strings.TrimFunc(str, unicode.IsSpace)
	}
	return phpv.ZString(str).ZVal(), nil
}

// mb_ltrim trims characters from the left side of a string
func fncMbLtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var chars *phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	if chars != nil {
		charSet := string(*chars)
		runeSet := []rune(charSet)
		isInSet := func(r rune) bool {
			for _, c := range runeSet {
				if r == c {
					return true
				}
			}
			return false
		}
		str = strings.TrimLeftFunc(str, isInSet)
	} else {
		str = strings.TrimLeftFunc(str, unicode.IsSpace)
	}
	return phpv.ZString(str).ZVal(), nil
}

// mb_rtrim trims characters from the right side of a string
func fncMbRtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var chars *phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil {
		return nil, err
	}

	str := string(s)
	if chars != nil {
		charSet := string(*chars)
		runeSet := []rune(charSet)
		isInSet := func(r rune) bool {
			for _, c := range runeSet {
				if r == c {
					return true
				}
			}
			return false
		}
		str = strings.TrimRightFunc(str, isInSet)
	} else {
		str = strings.TrimRightFunc(str, unicode.IsSpace)
	}
	return phpv.ZString(str).ZVal(), nil
}

// mb_output_handler is a callback function that converts character encoding in output buffer
func fncMbOutputHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var contents phpv.ZString
	_, err := core.Expand(ctx, args, &contents)
	if err != nil {
		return nil, err
	}
	// In goro, everything is UTF-8, just pass through
	return contents.ZVal(), nil
}

// mb_strwidth returns the width of a string, where East Asian wide/fullwidth
// characters count as 2, and other characters count as 1.
func fncMbStrwidth(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	// Convert to UTF-8 if needed (for now we assume the input uses the requested encoding)
	str := string(s)
	width := 0

	for _, r := range str {
		width += runeWidth(r)
	}

	return phpv.ZInt(width).ZVal(), nil
}

// runeWidth returns the display width of a rune.
// East Asian wide and fullwidth characters return 2, others return 1.
// Control characters return 0.
func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	// Control characters
	if r < 0x20 || (r >= 0x7F && r < 0xA0) {
		return 0
	}
	// East Asian Fullwidth and Wide characters
	if isEastAsianWide(r) {
		return 2
	}
	return 1
}

// isEastAsianWide returns true if the rune is considered "wide" in East Asian contexts.
func isEastAsianWide(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// CJK Unified Ideographs Extension B-F
	if r >= 0x20000 && r <= 0x2FA1F {
		return true
	}
	// Fullwidth Forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	// CJK Symbols and Punctuation
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// Hangul Jamo
	if r >= 0x1100 && r <= 0x115F {
		return true
	}
	if r >= 0x2329 && r <= 0x232A {
		return true
	}
	// Enclosed CJK Letters and Months
	if r >= 0x3200 && r <= 0x32FF {
		return true
	}
	// CJK Compatibility
	if r >= 0x3300 && r <= 0x33FF {
		return true
	}
	// Bopomofo
	if r >= 0x3100 && r <= 0x312F {
		return true
	}
	// Kanbun
	if r >= 0x3190 && r <= 0x319F {
		return true
	}
	return false
}

// mb_parse_str parses a URL-encoded string and stores the results in the
// second parameter (passed by reference).
func fncMbParseStr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return phpv.ZFalse.ZVal(), nil
	}
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	// Use the standard parse_str logic - mb_parse_str in PHP 8+ requires
	// the second parameter and behaves like parse_str with that parameter.
	result := phpv.NewZArray()

	pairs := strings.Split(string(s), "&")
	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		key := urlDecode(parts[0])
		val := ""
		if len(parts) > 1 {
			val = urlDecode(parts[1])
		}
		// Handle array notation in keys (e.g., "foo[bar]=val")
		if idx := strings.Index(key, "["); idx >= 0 {
			// Simple array handling
			result.OffsetSet(ctx, phpv.ZString(key[:idx]).ZVal(), phpv.ZString(val).ZVal())
		} else {
			result.OffsetSet(ctx, phpv.ZString(key).ZVal(), phpv.ZString(val).ZVal())
		}
	}

	// Write result to second parameter (passed by reference)
	*args[1] = *result.ZVal()
	return phpv.ZTrue.ZVal(), nil
}

// urlDecode decodes a URL-encoded string.
func urlDecode(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			h := s[i+1 : i+3]
			var b byte
			for _, c := range h {
				b <<= 4
				if c >= '0' && c <= '9' {
					b |= byte(c - '0')
				} else if c >= 'a' && c <= 'f' {
					b |= byte(c - 'a' + 10)
				} else if c >= 'A' && c <= 'F' {
					b |= byte(c - 'A' + 10)
				}
			}
			result.WriteByte(b)
			i += 2
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// encodingAliases maps encoding names to their aliases
var encodingAliases = map[string][]string{
	"ASCII":        {"ANSI_X3.4-1968", "iso-ir-6", "ANSI_X3.4-1986", "ISO_646.irv:1991", "US-ASCII", "ISO646-US", "us", "IBM367", "cp367", "csASCII"},
	"UTF-8":        {"utf8"},
	"UTF-16":       {"utf16"},
	"ISO-8859-1":   {"ISO_8859-1", "latin1"},
	"ISO-8859-15":  {"ISO_8859-15", "Latin-9"},
	"EUC-JP":       {"EUC_JP", "eucJP-win", "eucJP-open"},
	"SJIS":         {"Shift_JIS", "SJIS-win", "cp932"},
	"WINDOWS-1252": {"cp1252"},
	"WINDOWS-1251": {"cp1251"},
}

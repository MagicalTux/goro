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

func fncMbGetInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var optType *phpv.ZString
	_, err := core.Expand(ctx, args, &optType)
	if err != nil {
		return nil, err
	}
	internalEnc := getMbInternalEncoding(ctx)
	illegalChars := getIllegalChars(ctx)
	language := ctx.GetConfig("mbstring.language", phpv.ZString("neutral").ZVal()).String()
	if language == "" || language == `"neutral"` { language = "neutral" }
	encodingTranslation := ctx.GetConfig("mbstring.encoding_translation", phpv.ZString("0").ZVal()).String()
	if encodingTranslation == "1" || strings.ToLower(encodingTranslation) == "on" { encodingTranslation = "On" } else { encodingTranslation = "Off" }
	strictDetection := ctx.GetConfig("mbstring.strict_detection", phpv.ZString("0").ZVal()).String()
	if strictDetection == "1" || strings.ToLower(strictDetection) == "on" { strictDetection = "On" } else { strictDetection = "Off" }
	subChar := getMbSubstituteCharacter(ctx)
	var subCharVal *phpv.ZVal
	switch v := subChar.(type) {
	case string: subCharVal = phpv.ZString(v).ZVal()
	case int64: subCharVal = phpv.ZInt(v).ZVal()
	default: subCharVal = phpv.ZInt(63).ZVal()
	}
	detectOrder := getDetectOrder(ctx)
	detectOrderArr := phpv.NewZArray()
	for _, enc := range detectOrder { detectOrderArr.OffsetSet(ctx, nil, phpv.ZString(enc).ZVal()) }
	mailCharset, mailHeaderEnc, mailBodyEnc := "UTF-8", "BASE64", "BASE64"
	switch strings.ToLower(language) {
	case "japanese": mailCharset, mailHeaderEnc, mailBodyEnc = "ISO-2022-JP", "BASE64", "7bit"
	case "korean": mailCharset, mailHeaderEnc, mailBodyEnc = "ISO-2022-KR", "BASE64", "7bit"
	case "english": mailCharset, mailHeaderEnc, mailBodyEnc = "ISO-8859-1", "BASE64", "8bit"
	case "german": mailCharset, mailHeaderEnc, mailBodyEnc = "ISO-8859-15", "BASE64", "8bit"
	}
	if optType != nil {
		switch strings.ToLower(string(*optType)) {
		case "internal_encoding": return phpv.ZString(internalEnc).ZVal(), nil
		case "http_input": return phpv.ZString("").ZVal(), nil
		case "http_output": return phpv.ZString("UTF-8").ZVal(), nil
		case "func_overload": return phpv.ZInt(0).ZVal(), nil
		case "mail_charset": return phpv.ZString(mailCharset).ZVal(), nil
		case "mail_header_encoding": return phpv.ZString(mailHeaderEnc).ZVal(), nil
		case "mail_body_encoding": return phpv.ZString(mailBodyEnc).ZVal(), nil
		case "illegal_chars": return phpv.ZInt(illegalChars).ZVal(), nil
		case "detect_order": return detectOrderArr.ZVal(), nil
		case "language": return phpv.ZString(language).ZVal(), nil
		case "encoding_translation": return phpv.ZString(encodingTranslation).ZVal(), nil
		case "substitute_character": return subCharVal, nil
		case "strict_detection": return phpv.ZString(strictDetection).ZVal(), nil
		default: return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_get_info(): Argument #1 ($type) must be a valid info type")
		}
	}
	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZString("internal_encoding").ZVal(), phpv.ZString(internalEnc).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("http_input").ZVal(), phpv.ZString("").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("http_output").ZVal(), phpv.ZString("UTF-8").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("http_output_conv_mimetypes").ZVal(), phpv.ZString("^(text/|application/xhtml\\+xml)").ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_charset").ZVal(), phpv.ZString(mailCharset).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_header_encoding").ZVal(), phpv.ZString(mailHeaderEnc).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("mail_body_encoding").ZVal(), phpv.ZString(mailBodyEnc).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("illegal_chars").ZVal(), phpv.ZInt(illegalChars).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("encoding_translation").ZVal(), phpv.ZString(encodingTranslation).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("language").ZVal(), phpv.ZString(language).ZVal())
	arr.OffsetSet(ctx, phpv.ZString("detect_order").ZVal(), detectOrderArr.ZVal())
	arr.OffsetSet(ctx, phpv.ZString("substitute_character").ZVal(), subCharVal)
	arr.OffsetSet(ctx, phpv.ZString("strict_detection").ZVal(), phpv.ZString(strictDetection).ZVal())
	return arr.ZVal(), nil
}

func fncMbScrub(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil { return nil, err }
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_scrub(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	str := string(s)
	if utf8.ValidString(str) { return s.ZVal(), nil }
	var result strings.Builder
	for i := 0; i < len(str); {
		r, size := utf8.DecodeRuneInString(str[i:])
		if r == utf8.RuneError && size == 1 { result.WriteRune(0xFFFD); i++ } else { result.WriteRune(r); i += size }
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func fncMbStripos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil { return nil, err }
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_stripos(): Argument #4 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	hRunes := []rune(strings.ToLower(string(haystack)))
	nRunes := []rune(strings.ToLower(string(needle)))
	if len(nRunes) == 0 {
		start := 0
		if offset != nil { start = int(*offset); if start < 0 { start = len(hRunes) + start } }
		if start < 0 || start > len(hRunes) { return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_stripos(): Argument #3 ($offset) must be contained in argument #1 ($haystack)") }
		return phpv.ZInt(start).ZVal(), nil
	}
	start := 0
	if offset != nil { start = int(*offset); if start < 0 { start = len(hRunes) + start }; if start < 0 || start > len(hRunes) { return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_stripos(): Argument #3 ($offset) must be contained in argument #1 ($haystack)") } }
	for i := start; i <= len(hRunes)-len(nRunes); i++ {
		match := true
		for j := 0; j < len(nRunes); j++ { if hRunes[i+j] != nRunes[j] { match = false; break } }
		if match { return phpv.ZInt(i).ZVal(), nil }
	}
	return phpv.ZBool(false).ZVal(), nil
}

func fncMbStrripos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil { return nil, err }
	hRunes := []rune(strings.ToLower(string(haystack)))
	nRunes := []rune(strings.ToLower(string(needle)))
	if len(nRunes) == 0 {
		start := len(hRunes)
		if offset != nil { o := int(*offset); if o < 0 { start = len(hRunes) + o } }
		return phpv.ZInt(start).ZVal(), nil
	}
	start := len(hRunes) - 1
	searchFrom := 0
	if offset != nil { o := int(*offset); if o >= 0 { searchFrom = o } else { start = len(hRunes) + o } }
	for i := start; i >= searchFrom; i-- {
		if i+len(nRunes) > len(hRunes) { continue }
		match := true
		for j := 0; j < len(nRunes); j++ { if hRunes[i+j] != nRunes[j] { match = false; break } }
		if match { return phpv.ZInt(i).ZVal(), nil }
	}
	return phpv.ZBool(false).ZVal(), nil
}

func fncMbStrrchr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString; var beforeNeedle *phpv.ZBool; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &beforeNeedle, &enc)
	if err != nil { return nil, err }
	before := core.Deref(beforeNeedle, false)
	h, n := string(haystack), string(needle)
	if len(n) > 0 { r, _ := utf8.DecodeRuneInString(n); n = string(r) }
	idx := strings.LastIndex(h, n)
	if idx < 0 { return phpv.ZFalse.ZVal(), nil }
	if bool(before) { return phpv.ZString(h[:idx]).ZVal(), nil }
	return phpv.ZString(h[idx:]).ZVal(), nil
}

func fncMbStrrichr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString; var beforeNeedle *phpv.ZBool; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &beforeNeedle, &enc)
	if err != nil { return nil, err }
	before := core.Deref(beforeNeedle, false)
	h, n := string(haystack), string(needle)
	if len(n) > 0 { r, _ := utf8.DecodeRuneInString(n); n = string(r) }
	idx := strings.LastIndex(strings.ToLower(h), strings.ToLower(n))
	if idx < 0 { return phpv.ZFalse.ZVal(), nil }
	if bool(before) { return phpv.ZString(h[:idx]).ZVal(), nil }
	return phpv.ZString(h[idx:]).ZVal(), nil
}

func fncMbStrimwidth(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var start, width phpv.ZInt; var trimmarker, enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &start, &width, &trimmarker, &enc)
	if err != nil { return nil, err }
	runes := []rune(string(s)); runeLen := len(runes)
	st := int(start); if st < 0 { st = runeLen + st }; if st < 0 { st = 0 }; if st > runeLen { return phpv.ZString("").ZVal(), nil }
	w := int(width); if w < 0 { w = runeLen + w - st; if w < 0 { w = 0 } }
	marker := ""; if trimmarker != nil { marker = string(*trimmarker) }; markerRunes := []rune(marker)
	remaining := runes[st:]
	if len(remaining) <= w { return phpv.ZString(string(remaining)).ZVal(), nil }
	if len(markerRunes) >= w { return phpv.ZString(string(markerRunes[:w])).ZVal(), nil }
	cutLen := w - len(markerRunes)
	return phpv.ZString(string(remaining[:cutLen]) + marker).ZVal(), nil
}

func fncMbStrcut(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var start phpv.ZInt; var length *phpv.ZInt; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &start, &length, &enc)
	if err != nil { return nil, err }
	str := string(s); st := int(start)
	if st < 0 { st = len(str) + st }; if st < 0 { st = 0 }; if st > len(str) { return phpv.ZString("").ZVal(), nil }
	for st > 0 && st < len(str) && !utf8.RuneStart(str[st]) { st-- }
	end := len(str)
	if length != nil { l := int(*length); if l < 0 { end = len(str) + l } else { end = st + l } }
	if end > len(str) { end = len(str) }; if end < st { return phpv.ZString("").ZVal(), nil }
	for end > st && end < len(str) && !utf8.RuneStart(str[end]) { end-- }
	return phpv.ZString(str[st:end]).ZVal(), nil
}

func fncMbConvertVariables(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 { return nil, ctx.Errorf("mb_convert_variables() expects at least 3 arguments") }
	return phpv.ZString("UTF-8").ZVal(), nil
}

func fncMbHttpInput(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZBool(false).ZVal(), nil }

func fncMbHttpOutput(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 { return phpv.ZString("UTF-8").ZVal(), nil }
	return phpv.ZBool(true).ZVal(), nil
}

func fncMbEncodingAliases(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var encoding phpv.ZString
	_, err := core.Expand(ctx, args, &encoding)
	if err != nil { return nil, err }
	if !isValidEncoding(string(encoding)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_encoding_aliases(): Argument #1 ($encoding) must be a valid encoding, \"%s\" given", string(encoding)))
	}
	enc := strings.ToUpper(string(encoding))
	aliases, ok := encodingAliases[enc]
	if !ok {
		n := strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", "")
		for k, v := range encodingAliases { if strings.ReplaceAll(strings.ReplaceAll(k, "-", ""), "_", "") == n { aliases = v; ok = true; break } }
	}
	if !ok { aliases = []string{} }
	arr := phpv.NewZArray()
	for _, a := range aliases { arr.OffsetSet(ctx, nil, phpv.ZString(a).ZVal()) }
	return arr.ZVal(), nil
}

func fncMbStrPad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var length phpv.ZInt; var padStr *phpv.ZString; var padType *phpv.ZInt; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &length, &padStr, &padType, &enc)
	if err != nil { return nil, err }
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_str_pad(): Argument #5 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	pad := " "; if padStr != nil { pad = string(*padStr) }
	pType := 1; if padType != nil { pType = int(*padType) }
	if pType != 0 && pType != 1 && pType != 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_str_pad(): Argument #4 ($pad_type) must be STR_PAD_LEFT, STR_PAD_RIGHT, or STR_PAD_BOTH")
	}
	if pad == "" { return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_str_pad(): Argument #3 ($pad_string) must not be empty") }
	sRunes := []rune(string(s)); padRunes := []rune(pad); targetLen := int(length)
	if len(sRunes) >= targetLen { return s.ZVal(), nil }
	needed := targetLen - len(sRunes)
	var result []rune
	switch pType {
	case 1: // STR_PAD_RIGHT
		result = append(append([]rune{}, sRunes...), make([]rune, needed)...)
		for i := 0; i < needed; i++ { result[len(sRunes)+i] = padRunes[i%len(padRunes)] }
	case 0: // STR_PAD_LEFT
		prefix := make([]rune, needed)
		for i := 0; i < needed; i++ { prefix[i] = padRunes[i%len(padRunes)] }
		result = append(prefix, sRunes...)
	case 2: // STR_PAD_BOTH
		leftPad := needed / 2; rightPad := needed - leftPad
		result = make([]rune, 0, targetLen)
		for i := 0; i < leftPad; i++ { result = append(result, padRunes[i%len(padRunes)]) }
		result = append(result, sRunes...)
		for i := 0; i < rightPad; i++ { result = append(result, padRunes[i%len(padRunes)]) }
	}
	return phpv.ZString(string(result)).ZVal(), nil
}

func fncMbTrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var chars *phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil { return nil, err }
	str := string(s)
	if chars != nil {
		runeSet := []rune(string(*chars))
		str = strings.TrimFunc(str, func(r rune) bool { for _, c := range runeSet { if r == c { return true } }; return false })
	} else { str = strings.TrimFunc(str, unicode.IsSpace) }
	return phpv.ZString(str).ZVal(), nil
}

func fncMbLtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var chars *phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil { return nil, err }
	str := string(s)
	if chars != nil {
		runeSet := []rune(string(*chars))
		str = strings.TrimLeftFunc(str, func(r rune) bool { for _, c := range runeSet { if r == c { return true } }; return false })
	} else { str = strings.TrimLeftFunc(str, unicode.IsSpace) }
	return phpv.ZString(str).ZVal(), nil
}

func fncMbRtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var chars *phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &chars, &enc)
	if err != nil { return nil, err }
	str := string(s)
	if chars != nil {
		runeSet := []rune(string(*chars))
		str = strings.TrimRightFunc(str, func(r rune) bool { for _, c := range runeSet { if r == c { return true } }; return false })
	} else { str = strings.TrimRightFunc(str, unicode.IsSpace) }
	return phpv.ZString(str).ZVal(), nil
}

func fncMbOutputHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var contents phpv.ZString
	_, err := core.Expand(ctx, args, &contents)
	if err != nil { return nil, err }
	return contents.ZVal(), nil
}

func fncMbStrwidth(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil { return nil, err }
	width := 0; for _, r := range string(s) { width += runeWidth(r) }
	return phpv.ZInt(width).ZVal(), nil
}

func runeWidth(r rune) int {
	if r == 0 { return 0 }
	if r < 0x20 || (r >= 0x7F && r < 0xA0) { return 0 }
	if isEastAsianWide(r) { return 2 }
	return 1
}

func isEastAsianWide(r rune) bool {
	if r >= 0x4E00 && r <= 0x9FFF { return true }
	if r >= 0x3400 && r <= 0x4DBF { return true }
	if r >= 0xF900 && r <= 0xFAFF { return true }
	if r >= 0x20000 && r <= 0x2FA1F { return true }
	if r >= 0xFF01 && r <= 0xFF60 { return true }
	if r >= 0xFFE0 && r <= 0xFFE6 { return true }
	if r >= 0x30A0 && r <= 0x30FF { return true }
	if r >= 0x3040 && r <= 0x309F { return true }
	if r >= 0x3000 && r <= 0x303F { return true }
	if r >= 0xAC00 && r <= 0xD7AF { return true }
	if r >= 0x1100 && r <= 0x115F { return true }
	if r >= 0x2329 && r <= 0x232A { return true }
	if r >= 0x3200 && r <= 0x32FF { return true }
	if r >= 0x3300 && r <= 0x33FF { return true }
	if r >= 0x3100 && r <= 0x312F { return true }
	if r >= 0x3190 && r <= 0x319F { return true }
	return false
}

func fncMbParseStr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 { return nil, ctx.Errorf("mb_parse_str() expects exactly 2 arguments, %d given", len(args)) }
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil { return nil, err }
	result := phpv.NewZArray()
	for _, pair := range strings.Split(string(s), "&") {
		if pair == "" { continue }
		parts := strings.SplitN(pair, "=", 2)
		key := urlDecode(parts[0]); val := ""; if len(parts) > 1 { val = urlDecode(parts[1]) }
		result.OffsetSet(ctx, phpv.ZString(key).ZVal(), phpv.ZString(val).ZVal())
	}
	*args[1] = *result.ZVal()
	return phpv.ZTrue.ZVal(), nil
}

func urlDecode(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			h := s[i+1 : i+3]; var b byte
			for _, c := range h { b <<= 4; if c >= '0' && c <= '9' { b |= byte(c - '0') } else if c >= 'a' && c <= 'f' { b |= byte(c - 'a' + 10) } else if c >= 'A' && c <= 'F' { b |= byte(c - 'A' + 10) } }
			result.WriteByte(b); i += 2
		} else { result.WriteByte(s[i]) }
	}
	return result.String()
}

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

func fncMbEncodeNumericentity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 { return nil, ctx.Errorf("mb_encode_numericentity() expects at least 2 arguments") }
	str := args[0].String(); convmap := args[1]
	if convmap.GetType() != phpv.ZtArray { return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_encode_numericentity(): Argument #2 ($map) must be of type array") }
	mapArr := convmap.Value().(*phpv.ZArray)
	var mapVals []int
	for _, v := range mapArr.Iterate(ctx) { mapVals = append(mapVals, int(v.AsInt(ctx))) }
	isHex := false; if len(args) > 3 && args[3] != nil { isHex = args[3].AsBool(ctx) == phpv.ZTrue }
	var result strings.Builder
	for _, r := range str {
		encoded := false
		for i := 0; i+3 < len(mapVals); i += 4 {
			if int(r) >= mapVals[i] && int(r) <= mapVals[i+1] {
				cp := (int(r) + mapVals[i+2]) & mapVals[i+3]
				if isHex { result.WriteString(fmt.Sprintf("&#x%X;", cp)) } else { result.WriteString(fmt.Sprintf("&#%d;", cp)) }
				encoded = true; break
			}
		}
		if !encoded { result.WriteRune(r) }
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func fncMbDecodeNumericentity(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 { return nil, ctx.Errorf("mb_decode_numericentity() expects at least 2 arguments") }
	str := args[0].String(); convmap := args[1]
	if convmap.GetType() != phpv.ZtArray { return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_decode_numericentity(): Argument #2 ($map) must be of type array") }
	mapArr := convmap.Value().(*phpv.ZArray)
	var mapVals []int
	for _, v := range mapArr.Iterate(ctx) { mapVals = append(mapVals, int(v.AsInt(ctx))) }
	var result strings.Builder
	i := 0
	for i < len(str) {
		if str[i] == '&' && i+2 < len(str) && str[i+1] == '#' {
			j := i + 2; isHex := false
			if j < len(str) && (str[j] == 'x' || str[j] == 'X') { isHex = true; j++ }
			numStart := j
			for j < len(str) && ((str[j] >= '0' && str[j] <= '9') || (isHex && ((str[j] >= 'a' && str[j] <= 'f') || (str[j] >= 'A' && str[j] <= 'F')))) { j++ }
			if j > numStart && j < len(str) && str[j] == ';' {
				var cp int
				if isHex { fmt.Sscanf(str[numStart:j], "%x", &cp) } else { fmt.Sscanf(str[numStart:j], "%d", &cp) }
				decoded := false
				for k := 0; k+3 < len(mapVals); k += 4 {
					orig := (cp & mapVals[k+3]) - mapVals[k+2]
					if orig >= mapVals[k] && orig <= mapVals[k+1] { result.WriteRune(rune(orig)); decoded = true; break }
				}
				if !decoded { result.WriteString(str[i : j+1]) }
				i = j + 1; continue
			}
		}
		result.WriteByte(str[i]); i++
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func fncMbDecodeMimeheader(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil { return nil, err }
	str := string(s); var result strings.Builder; i := 0
	for i < len(str) {
		if i+2 < len(str) && str[i] == '=' && str[i+1] == '?' {
			end := strings.Index(str[i+2:], "?=")
			if end >= 0 {
				parts := strings.SplitN(str[i+2:i+2+end], "?", 3)
				if len(parts) == 3 {
					charset, encType, text := parts[0], strings.ToUpper(parts[1]), parts[2]
					var decoded string
					if encType == "B" { decoded = mimeDecBase64(text) } else if encType == "Q" { decoded = mimeDecQP(text) } else { decoded = text }
					if n := normalizeEncodingName(charset); n != "UTF-8" && n != "UTF8" { c, _, _ := decodeToUTF8([]byte(decoded), n); result.Write(c) } else { result.WriteString(decoded) }
					i = i + 2 + end + 2; continue
				}
			}
		}
		result.WriteByte(str[i]); i++
	}
	return phpv.ZString(result.String()).ZVal(), nil
}

func mimeDecBase64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte; var buf uint32; var bits int
	for _, c := range s { if c == '=' { break }; idx := strings.IndexRune(chars, c); if idx < 0 { continue }; buf = (buf << 6) | uint32(idx); bits += 6; if bits >= 8 { bits -= 8; result = append(result, byte(buf>>uint(bits))); buf &= (1 << uint(bits)) - 1 } }
	return string(result)
}

func mimeDecQP(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '=' && i+2 < len(s) {
			var b byte; valid := true
			for _, c := range s[i+1 : i+3] { b <<= 4; if c >= '0' && c <= '9' { b |= byte(c - '0') } else if c >= 'a' && c <= 'f' { b |= byte(c - 'a' + 10) } else if c >= 'A' && c <= 'F' { b |= byte(c - 'A' + 10) } else { valid = false; break } }
			if valid { result = append(result, b); i += 2; continue }
		}
		if s[i] == '_' { result = append(result, ' ') } else { result = append(result, s[i]) }
	}
	return string(result)
}

func fncMbEncodeMimeheader(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var charset, transferEnc, linefeed *phpv.ZString; var indent *phpv.ZInt
	_, err := core.Expand(ctx, args, &s, &charset, &transferEnc, &linefeed, &indent)
	if err != nil { return nil, err }
	str := string(s); cs := "UTF-8"; if charset != nil { cs = string(*charset) }
	needsEnc := false; for i := 0; i < len(str); i++ { if str[i] > 127 { needsEnc = true; break } }
	if !needsEnc { return phpv.ZString(str).ZVal(), nil }
	enc := "B"; if transferEnc != nil { enc = strings.ToUpper(string(*transferEnc)) }
	var encoded string
	if enc == "Q" {
		var r strings.Builder
		for i := 0; i < len(str); i++ {
			if s[i] == ' ' { r.WriteByte('_') } else if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= '0' && s[i] <= '9') { r.WriteByte(s[i]) } else { r.WriteString(fmt.Sprintf("=%02X", s[i])) }
		}
		encoded = r.String()
	} else {
		const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		data := []byte(str); var r strings.Builder
		for i := 0; i < len(data); i += 3 {
			var b0, b1, b2 byte; b0 = data[i]; if i+1 < len(data) { b1 = data[i+1] }; if i+2 < len(data) { b2 = data[i+2] }
			r.WriteByte(b64[b0>>2]); r.WriteByte(b64[((b0&3)<<4)|(b1>>4)])
			if i+1 < len(data) { r.WriteByte(b64[((b1&0xF)<<2)|(b2>>6)]) } else { r.WriteByte('=') }
			if i+2 < len(data) { r.WriteByte(b64[b2&0x3F]) } else { r.WriteByte('=') }
		}
		encoded = r.String()
	}
	return phpv.ZString(fmt.Sprintf("=?%s?%s?%s?=", cs, enc, encoded)).ZVal(), nil
}

func fncMbConvertKana(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var option, enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &option, &enc)
	if err != nil { return nil, err }
	return s.ZVal(), nil
}

func fncMbRegexSetOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 { return phpv.ZString("msr").ZVal(), nil }
	return phpv.ZString(args[0].String()).ZVal(), nil
}

func fncMbUcfirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil { return nil, err }
	str := string(s); if len(str) == 0 { return s.ZVal(), nil }
	runes := []rune(str); runes[0] = unicode.ToUpper(runes[0])
	return phpv.ZString(string(runes)).ZVal(), nil
}

func fncMbLcfirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString; var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil { return nil, err }
	str := string(s); if len(str) == 0 { return s.ZVal(), nil }
	runes := []rune(str); runes[0] = unicode.ToLower(runes[0])
	return phpv.ZString(string(runes)).ZVal(), nil
}

package mbstring

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func getMbInternalEncoding(ctx phpv.Context) string {
	v := ctx.GetConfig("mbstring.internal_encoding", phpv.ZNULL.ZVal())
	s := v.String()
	if s == "" || s == "NULL" {
		return "UTF-8"
	}
	return getCanonicalEncodingName(s)
}

func getMbSubstituteCharacter(ctx phpv.Context) interface{} {
	v := ctx.GetConfig("mbstring.substitute_character", phpv.ZNULL.ZVal())
	s := v.String()
	if s == "" || s == "NULL" {
		return int64(63)
	}
	switch strings.ToLower(s) {
	case "none":
		return "none"
	case "long":
		return "long"
	case "entity":
		return "entity"
	default:
		return int64(v.AsInt(ctx))
	}
}

func resolveEncoding(ctx phpv.Context, enc *phpv.ZString) string {
	if enc != nil && string(*enc) != "" {
		return getCanonicalEncodingName(string(*enc))
	}
	return getMbInternalEncoding(ctx)
}

func incrementIllegalChars(ctx phpv.Context, count int) {
	current := ctx.GetConfig("mbstring._illegal_chars", phpv.ZInt(0).ZVal())
	n := int(current.AsInt(ctx))
	ctx.Global().SetLocalConfig("mbstring._illegal_chars", phpv.ZInt(n+count).ZVal())
}

func getIllegalChars(ctx phpv.Context) int {
	v := ctx.GetConfig("mbstring._illegal_chars", phpv.ZInt(0).ZVal())
	return int(v.AsInt(ctx))
}

func getDetectOrder(ctx phpv.Context) []string {
	v := ctx.GetConfig("mbstring.detect_order", phpv.ZNULL.ZVal())
	s := v.String()
	if s == "" || s == "NULL" {
		return []string{"ASCII", "UTF-8"}
	}
	var result []string
	for _, e := range strings.Split(s, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			result = append(result, getCanonicalEncodingName(e))
		}
	}
	if len(result) == 0 {
		return []string{"ASCII", "UTF-8"}
	}
	return result
}

func detectFromEncodings(str string, encodings []string) string {
	for _, enc := range encodings {
		n := normalizeEncodingName(enc)
		switch n {
		case "ASCII":
			ok := true
			for i := 0; i < len(str); i++ {
				if str[i] > 127 {
					ok = false
					break
				}
			}
			if ok {
				return enc
			}
		case "UTF-8", "UTF8":
			if utf8.ValidString(str) {
				return enc
			}
		default:
			if isCheckEncodingValid(str, n) {
				return enc
			}
		}
	}
	if len(encodings) > 0 {
		return encodings[0]
	}
	return "UTF-8"
}

func fncMbStrlen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_strlen(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	encoding := resolveEncoding(ctx, enc)
	return phpv.ZInt(mbStrlen(string(s), encoding)).ZVal(), nil
}

func fncMbStrpos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil {
		return nil, err
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_strpos(): Argument #4 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	hRunes := []rune(string(haystack))
	nRunes := []rune(string(needle))
	if len(nRunes) == 0 {
		start := 0
		if offset != nil {
			start = int(*offset)
			if start < 0 {
				start = len(hRunes) + start
			}
		}
		if start < 0 || start > len(hRunes) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_strpos(): Argument #3 ($offset) must be contained in argument #1 ($haystack)")
		}
		return phpv.ZInt(start).ZVal(), nil
	}
	start := 0
	if offset != nil {
		start = int(*offset)
		if start < 0 {
			start = len(hRunes) + start
		}
		if start < 0 || start > len(hRunes) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_strpos(): Argument #3 ($offset) must be contained in argument #1 ($haystack)")
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

func fncMbStrrpos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil {
		return nil, err
	}
	hRunes := []rune(string(haystack))
	nRunes := []rune(string(needle))
	if len(nRunes) == 0 {
		start := len(hRunes)
		if offset != nil {
			o := int(*offset)
			if o < 0 {
				start = len(hRunes) + o
			}
		}
		return phpv.ZInt(start).ZVal(), nil
	}
	start := len(hRunes) - 1
	searchFrom := 0
	if offset != nil {
		o := int(*offset)
		if o >= 0 {
			searchFrom = o
		} else {
			start = len(hRunes) + o
		}
	}
	for i := start; i >= searchFrom; i-- {
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

func fncMbSubstr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var start phpv.ZInt
	var length *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &start, &length, &enc)
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
	var end int
	if length == nil {
		end = runeLen
	} else {
		l := int(*length)
		if l < 0 {
			end = runeLen + l
		} else {
			end = st + l
		}
	}
	if end > runeLen {
		end = runeLen
	}
	if end < st {
		return phpv.ZString("").ZVal(), nil
	}
	return phpv.ZString(string(runes[st:end])).ZVal(), nil
}

func fncMbStrtolower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_strtolower(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	return phpv.ZString(strings.ToLower(string(s))).ZVal(), nil
}

func fncMbStrtoupper(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_strtoupper(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	return phpv.ZString(strings.ToUpper(string(s))).ZVal(), nil
}

func fncMbInternalEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &enc)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return phpv.ZString(getMbInternalEncoding(ctx)).ZVal(), nil
	}
	encStr := string(*enc)
	if !isValidEncoding(encStr) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_internal_encoding(): Argument #1 ($encoding) must be a valid encoding, \"%s\" given", encStr))
	}
	ctx.Global().SetLocalConfig("mbstring.internal_encoding", phpv.ZString(getCanonicalEncodingName(encStr)).ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

func fncMbDetectEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	if len(args) < 1 {
		return nil, ctx.Errorf("mb_detect_encoding() expects at least 1 argument")
	}
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	var encodings []string
	if len(args) > 1 && args[1] != nil && args[1].GetType() != phpv.ZtNull {
		arg := args[1]
		if arg.GetType() == phpv.ZtArray {
			arr := arg.Value().(*phpv.ZArray)
			for _, v := range arr.Iterate(ctx) {
				encName := v.String()
				if !isValidEncoding(encName) {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_detect_encoding(): Argument #2 ($encodings) contains invalid encoding \"%s\"", encName))
				}
				encodings = append(encodings, getCanonicalEncodingName(encName))
			}
		} else if arg.GetType() == phpv.ZtBool {
			// strict mode
		} else {
			encStr := arg.String()
			if encStr != "auto" && encStr != "AUTO" {
				for _, e := range strings.Split(encStr, ",") {
					e = strings.TrimSpace(e)
					if e == "" {
						continue
					}
					if !isValidEncoding(e) {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_detect_encoding(): Argument #2 ($encodings) contains invalid encoding \"%s\"", e))
					}
					encodings = append(encodings, getCanonicalEncodingName(e))
				}
			}
		}
		if len(encodings) == 0 && (args[1].GetType() != phpv.ZtBool) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_detect_encoding(): Argument #2 ($encodings) must specify at least one encoding")
		}
	}
	if len(encodings) == 0 {
		encodings = getDetectOrder(ctx)
	}
	str := string(s)
	for _, encName := range encodings {
		normalized := normalizeEncodingName(encName)
		switch normalized {
		case "ASCII":
			isASCII := true
			for i := 0; i < len(str); i++ {
				if str[i] > 127 {
					isASCII = false
					break
				}
			}
			if isASCII {
				return phpv.ZString("ASCII").ZVal(), nil
			}
		case "UTF-8", "UTF8":
			if utf8.ValidString(str) {
				return phpv.ZString("UTF-8").ZVal(), nil
			}
		default:
			if isCheckEncodingValid(str, normalized) {
				return phpv.ZString(getCanonicalEncodingName(encName)).ZVal(), nil
			}
		}
	}
	return phpv.ZBool(false).ZVal(), nil
}

func fncMbCheckEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZBool(true).ZVal(), nil
	}
	var encArg *phpv.ZString
	if len(args) > 1 {
		if err := core.ExpandAt(ctx, args, 1, &encArg); err != nil {
			return nil, err
		}
	}
	encoding := "UTF-8"
	if encArg != nil {
		encStr := string(*encArg)
		if !isValidEncoding(encStr) {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_check_encoding(): Argument #2 ($encoding) must be a valid encoding, \"%s\" given", encStr))
		}
		encoding = normalizeEncodingName(encStr)
	} else {
		encoding = normalizeEncodingName(getMbInternalEncoding(ctx))
	}
	firstArg := args[0]
	if firstArg == nil || firstArg.GetType() == phpv.ZtNull {
		return phpv.ZBool(true).ZVal(), nil
	}
	if firstArg.GetType() == phpv.ZtArray {
		arr, ok := firstArg.Value().(*phpv.ZArray)
		if !ok {
			return phpv.ZBool(false).ZVal(), nil
		}
		for k, v := range arr.Iterate(ctx) {
			if k.GetType() == phpv.ZtString && !isCheckEncodingValid(k.String(), encoding) {
				return phpv.ZBool(false).ZVal(), nil
			}
			if v.GetType() == phpv.ZtString && !isCheckEncodingValid(v.String(), encoding) {
				return phpv.ZBool(false).ZVal(), nil
			}
		}
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(isCheckEncodingValid(firstArg.String(), encoding)).ZVal(), nil
}

func fncMbConvertEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.Errorf("mb_convert_encoding() expects at least 2 arguments")
	}
	toEnc := args[1].String()
	if !isValidEncoding(toEnc) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_convert_encoding(): Argument #2 ($to_encoding) must be a valid encoding, \"%s\" given", toEnc))
	}
	var fromEncodings []string
	if len(args) > 2 && args[2] != nil && args[2].GetType() != phpv.ZtNull {
		fromArg := args[2]
		if fromArg.GetType() == phpv.ZtArray {
			arr := fromArg.Value().(*phpv.ZArray)
			for _, v := range arr.Iterate(ctx) {
				e := strings.TrimSpace(v.String())
				if e != "" {
					fromEncodings = append(fromEncodings, getCanonicalEncodingName(e))
				}
			}
		} else {
			encStr := fromArg.String()
			if encStr == "auto" || encStr == "AUTO" {
				fromEncodings = getDetectOrder(ctx)
			} else {
				for _, e := range strings.Split(encStr, ",") {
					e = strings.TrimSpace(e)
					if e != "" {
						fromEncodings = append(fromEncodings, getCanonicalEncodingName(e))
					}
				}
			}
		}
		if len(fromEncodings) == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_convert_encoding(): Argument #3 ($from_encoding) must specify at least one encoding")
		}
	}
	if len(fromEncodings) == 0 {
		fromEncodings = []string{getMbInternalEncoding(ctx)}
	}
	if args[0].GetType() == phpv.ZtArray {
		return mbConvertEncodingArray(ctx, args[0], toEnc, fromEncodings)
	}
	str := args[0].String()
	fromEnc := fromEncodings[0]
	if len(fromEncodings) > 1 {
		fromEnc = detectFromEncodings(str, fromEncodings)
	}
	result, illegal, _ := convertEncoding([]byte(str), fromEnc, toEnc)
	if illegal > 0 {
		incrementIllegalChars(ctx, illegal)
	}
	return phpv.ZString(result).ZVal(), nil
}

func mbConvertEncodingArray(ctx phpv.Context, arr *phpv.ZVal, toEnc string, fromEncodings []string) (*phpv.ZVal, error) {
	a, ok := arr.Value().(*phpv.ZArray)
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}
	result := phpv.NewZArray()
	for k, v := range a.Iterate(ctx) {
		if v.GetType() == phpv.ZtString {
			fromEnc := fromEncodings[0]
			if len(fromEncodings) > 1 {
				fromEnc = detectFromEncodings(v.String(), fromEncodings)
			}
			converted, illegal, _ := convertEncoding([]byte(v.String()), fromEnc, toEnc)
			if illegal > 0 {
				incrementIllegalChars(ctx, illegal)
			}
			result.OffsetSet(ctx, k, phpv.ZString(converted).ZVal())
		} else if v.GetType() == phpv.ZtArray {
			sub, err := mbConvertEncodingArray(ctx, v, toEnc, fromEncodings)
			if err != nil {
				return nil, err
			}
			result.OffsetSet(ctx, k, sub)
		} else {
			result.OffsetSet(ctx, k, v)
		}
	}
	return result.ZVal(), nil
}

func fncMbSubstituteCharacter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		sub := getMbSubstituteCharacter(ctx)
		switch v := sub.(type) {
		case string:
			return phpv.ZString(v).ZVal(), nil
		case int64:
			return phpv.ZInt(v).ZVal(), nil
		default:
			return phpv.ZInt(63).ZVal(), nil
		}
	}
	arg := args[0]
	if arg.GetType() == phpv.ZtString {
		s := strings.ToLower(arg.String())
		switch s {
		case "none", "long", "entity":
			ctx.Global().SetLocalConfig("mbstring.substitute_character", phpv.ZString(s).ZVal())
			return phpv.ZBool(true).ZVal(), nil
		default:
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_substitute_character(): Argument #1 ($substitute_character) must be \"none\", \"long\", \"entity\" or a valid codepoint")
		}
	}
	cp := int(arg.AsInt(ctx))
	if cp < 0 || (cp > 0xD7FF && cp < 0xE000) || cp > 0x10FFFF {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_substitute_character(): Argument #1 ($substitute_character) must be \"none\", \"long\", \"entity\" or a valid codepoint")
	}
	ctx.Global().SetLocalConfig("mbstring.substitute_character", phpv.ZInt(cp).ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

func fncMbSubstrCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle, &enc)
	if err != nil {
		return nil, err
	}
	if string(needle) == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_substr_count(): Argument #2 ($needle) must not be empty")
	}
	return phpv.ZInt(strings.Count(string(haystack), string(needle))).ZVal(), nil
}

func fncMbDetectOrder(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		order := getDetectOrder(ctx)
		arr := phpv.NewZArray()
		for _, enc := range order {
			arr.OffsetSet(ctx, nil, phpv.ZString(enc).ZVal())
		}
		return arr.ZVal(), nil
	}
	arg := args[0]
	if arg.GetType() == phpv.ZtArray {
		a := arg.Value().(*phpv.ZArray)
		var encodings []string
		for _, v := range a.Iterate(ctx) {
			e := strings.TrimSpace(v.String())
			if e != "" {
				if !isValidEncoding(e) {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_detect_order(): Argument #1 ($encoding) contains invalid encoding \"%s\"", e))
				}
				encodings = append(encodings, getCanonicalEncodingName(e))
			}
		}
		if len(encodings) == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_detect_order(): Argument #1 ($encoding) must specify at least one encoding")
		}
		ctx.Global().SetLocalConfig("mbstring.detect_order", phpv.ZString(strings.Join(encodings, ",")).ZVal())
		return phpv.ZBool(true).ZVal(), nil
	}
	encStr := arg.String()
	if encStr == "auto" || encStr == "AUTO" {
		ctx.Global().SetLocalConfig("mbstring.detect_order", phpv.ZString("ASCII,UTF-8").ZVal())
		return phpv.ZBool(true).ZVal(), nil
	}
	var encodings []string
	for _, e := range strings.Split(encStr, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			if !isValidEncoding(e) {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_detect_order(): Argument #1 ($encoding) contains invalid encoding \"%s\"", e))
			}
			encodings = append(encodings, getCanonicalEncodingName(e))
		}
	}
	if len(encodings) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_detect_order(): Argument #1 ($encoding) must specify at least one encoding")
	}
	ctx.Global().SetLocalConfig("mbstring.detect_order", phpv.ZString(strings.Join(encodings, ",")).ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

func fncMbLanguage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		v := ctx.GetConfig("mbstring.language", phpv.ZString("neutral").ZVal())
		s := v.String()
		if s == "" || s == `"neutral"` {
			return phpv.ZString("neutral").ZVal(), nil
		}
		return phpv.ZString(s).ZVal(), nil
	}
	lang := strings.ToLower(args[0].String())
	validLangs := map[string]bool{
		"neutral": true, "japanese": true, "ja": true,
		"english": true, "en": true, "german": true, "de": true,
		"korean": true, "ko": true, "uni": true, "unicode": true,
	}
	if !validLangs[lang] {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_language(): Argument #1 ($language) must be a valid language, \"%s\" given", args[0].String()))
	}
	switch lang {
	case "ja":
		lang = "Japanese"
	case "en":
		lang = "English"
	case "de":
		lang = "German"
	case "ko":
		lang = "Korean"
	case "uni", "unicode":
		lang = "uni"
	default:
		if len(lang) > 0 {
			lang = strings.ToUpper(lang[:1]) + lang[1:]
		}
	}
	ctx.Global().SetLocalConfig("mbstring.language", phpv.ZString(lang).ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

func fncMbStrSplit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var length *phpv.ZInt
	var enc *phpv.ZString
	_, err := core.Expand(ctx, args, &s, &length, &enc)
	if err != nil {
		return nil, err
	}
	splitLen := 1
	if length != nil {
		splitLen = int(*length)
		if splitLen < 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "mb_str_split(): Argument #2 ($length) must be greater than 0")
		}
	}
	if enc != nil && !isValidEncoding(string(*enc)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("mb_str_split(): Argument #3 ($encoding) must be a valid encoding, \"%s\" given", string(*enc)))
	}
	runes := []rune(string(s))
	arr := phpv.NewZArray()
	for i := 0; i < len(runes); i += splitLen {
		end := i + splitLen
		if end > len(runes) {
			end = len(runes)
		}
		arr.OffsetSet(ctx, nil, phpv.ZString(string(runes[i:end])).ZVal())
	}
	if arr.Count(ctx) == 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
	}
	return arr.ZVal(), nil
}

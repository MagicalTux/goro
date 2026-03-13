package mbstring

import (
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// mb_strlen returns the number of characters (not bytes) in a string.
// For UTF-8, this counts Unicode code points.
func fncMbStrlen(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(utf8.RuneCountInString(string(s))).ZVal(), nil
}

// mb_strpos finds the position of the first occurrence of a string in another string.
func fncMbStrpos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	var offset *phpv.ZInt
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle, &offset, &enc)
	if err != nil {
		return nil, err
	}

	hRunes := []rune(string(haystack))
	nRunes := []rune(string(needle))

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

// mb_strrpos finds the position of the last occurrence of a string in another string.
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

	start := len(hRunes) - 1
	if offset != nil {
		o := int(*offset)
		if o >= 0 {
			start = len(hRunes) - 1
			// Search from offset forward, but find last occurrence
		} else {
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

// mb_substr returns part of a string.
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

// mb_strtolower converts a string to lowercase (UTF-8 aware).
func fncMbStrtolower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(strings.ToLower(string(s))).ZVal(), nil
}

// mb_strtoupper converts a string to uppercase (UTF-8 aware).
func fncMbStrtoupper(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(strings.ToUpper(string(s))).ZVal(), nil
}

// mb_internal_encoding gets/sets the internal encoding.
func fncMbInternalEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &enc)
	if err != nil {
		return nil, err
	}

	if enc == nil {
		// Get current encoding — always UTF-8 in goro
		return phpv.ZString("UTF-8").ZVal(), nil
	}

	// Set encoding — we only really support UTF-8
	e := strings.ToUpper(string(*enc))
	if e == "UTF-8" || e == "UTF8" {
		return phpv.ZBool(true).ZVal(), nil
	}
	// Accept but ignore other encodings
	return phpv.ZBool(true).ZVal(), nil
}

// mb_detect_encoding detects encoding of a string.
func fncMbDetectEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString

	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	// In goro, strings are always UTF-8
	if utf8.ValidString(string(s)) {
		return phpv.ZString("UTF-8").ZVal(), nil
	}
	return phpv.ZString("ASCII").ZVal(), nil
}

// mb_check_encoding checks if strings are valid for the specified encoding.
func fncMbCheckEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s *phpv.ZString
	var enc *phpv.ZString

	_, err := core.Expand(ctx, args, &s, &enc)
	if err != nil {
		return nil, err
	}

	if s == nil {
		return phpv.ZBool(true).ZVal(), nil
	}

	return phpv.ZBool(utf8.ValidString(string(*s))).ZVal(), nil
}

// mb_convert_encoding converts encoding of a string.
func fncMbConvertEncoding(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var toEnc phpv.ZString

	_, err := core.Expand(ctx, args, &s, &toEnc)
	if err != nil {
		return nil, err
	}

	// In goro, everything is UTF-8 internally, so just return as-is
	return phpv.ZString(s).ZVal(), nil
}

// mb_substitute_character gets/sets substitution character.
func fncMbSubstituteCharacter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZString("none").ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

// mb_substr_count counts the number of substring occurrences.
func fncMbSubstrCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle)
	if err != nil {
		return nil, err
	}

	count := strings.Count(string(haystack), string(needle))
	return phpv.ZInt(count).ZVal(), nil
}

// mb_detect_order gets/sets encoding detection order.
func fncMbDetectOrder(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		// Return current order
		arr := phpv.NewZArray()
		arr.OffsetSet(ctx, nil, phpv.ZString("ASCII").ZVal())
		arr.OffsetSet(ctx, nil, phpv.ZString("UTF-8").ZVal())
		return arr.ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

// mb_language gets/sets the language.
func fncMbLanguage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return phpv.ZString("neutral").ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

// mb_str_split splits a multibyte string into an array of characters.
func fncMbStrSplit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	var length *phpv.ZInt

	_, err := core.Expand(ctx, args, &s, &length)
	if err != nil {
		return nil, err
	}

	splitLen := 1
	if length != nil {
		splitLen = int(*length)
		if splitLen < 1 {
			return nil, ctx.FuncErrorf("mb_str_split(): Argument #2 ($length) must be greater than 0")
		}
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

	return arr.ZVal(), nil
}

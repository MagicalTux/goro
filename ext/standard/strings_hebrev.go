package standard

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	_HEB_BLOCK_TYPE_ENG = 1
	_HEB_BLOCK_TYPE_HEB = 2
)

// translated from hebrev fround in ext/standard/strings.c
func hebrev(text string, max_chars int) string {
	isHebrew := func(c byte) bool {
		return c >= 224 && c <= 250
	}
	isBlank := func(c byte) bool {
		return c == ' ' || c == '\t'
	}
	isNewLine := func(c byte) bool {
		return c == '\n' || c == '\r'
	}
	isPunct := func(c byte) bool {
		// unicode.IsPunct is not used since it's defined differently
		switch c {
		case '!', '"', '#', '$', '%', '&', '\'', '(', ')', '*',
			'+', ',', '-', '.', '/', ':', ';', '<', '=', '>',
			'?', '@', '[', '\\', ']', '^', '_', '`', '{', '|',
			'}', '~':
			return true
		}
		return false
	}

	if len(text) == 0 {
		return ""
	}

	var blockType int
	if isHebrew(text[0]) {
		blockType = _HEB_BLOCK_TYPE_HEB
	} else {
		blockType = _HEB_BLOCK_TYPE_ENG
	}

	tmp_i := 0
	str := []byte(text)
	hebStr := make([]byte, len(str))
	target := hebStr
	target_i := len(hebStr) - 1

	blockStart := 0
	blockEnd := 0
	blockLen := 0
	strLen := len(str)

	for {
		if blockType == _HEB_BLOCK_TYPE_HEB {
			for tmp_i+1 < len(str) && (isHebrew(str[tmp_i+1]) || isBlank(str[tmp_i+1]) || isPunct(str[tmp_i+1]) || (str[tmp_i+1]) == '\n') && blockEnd < strLen-1 {
				tmp_i++
				blockEnd++
				blockLen++
			}
			for i := blockStart + 1; i <= blockEnd+1; i++ {
				target[target_i] = str[i-1]
				switch target[target_i] {
				case '(':
					target[target_i] = ')'
					break
				case ')':
					target[target_i] = '('
					break
				case '[':
					target[target_i] = ']'
					break
				case ']':
					target[target_i] = '['
					break
				case '{':
					target[target_i] = '}'
					break
				case '}':
					target[target_i] = '{'
					break
				case '<':
					target[target_i] = '>'
					break
				case '>':
					target[target_i] = '<'
					break
				case '\\':
					target[target_i] = '/'
					break
				case '/':
					target[target_i] = '\\'
					break
				default:
					break
				}
				target_i--
			}
			blockType = _HEB_BLOCK_TYPE_ENG
		} else {
			for tmp_i+1 < strLen && !isHebrew(str[tmp_i+1]) && str[tmp_i+1] != '\n' && blockEnd < strLen-1 {
				tmp_i++
				blockEnd++
				blockLen++
			}
			for (isBlank(str[tmp_i]) || isPunct(str[tmp_i])) && str[tmp_i] != '/' && str[tmp_i] != '-' && blockEnd > blockStart {
				tmp_i--
				blockEnd--
			}
			for i := blockEnd + 1; i >= blockStart+1; i-- {
				target[target_i] = str[i-1]
				target_i--
			}
			blockType = _HEB_BLOCK_TYPE_HEB
		}
		blockStart = blockEnd + 1

		if blockEnd >= strLen-1 {
			break
		}
	}

	end := strLen - 1
	begin := end

	result := make([]byte, strLen)
	result_i := 0

	for {
		charCount := 0
		for (max_chars == 0 || (max_chars > 0 && charCount < max_chars)) && begin > 0 {
			charCount++
			begin--
			if begin <= 0 || isNewLine(hebStr[begin]) {
				for begin > 0 && isNewLine(hebStr[begin-1]) {
					begin--
					charCount++
				}
				break
			}
		}
		if max_chars >= 0 && charCount == max_chars { /* try to avoid breaking words */
			newCharCount := charCount
			newBegin := begin

			for newCharCount > 0 {
				if isBlank(hebStr[newBegin]) || isNewLine(hebStr[newBegin]) {
					break
				}
				newBegin++
				newCharCount--
			}
			if newCharCount > 0 {
				begin = newBegin
			}
		}
		orinBegin := begin

		if isBlank(hebStr[begin]) {
			hebStr[begin] = '\n'
		}
		for begin <= end && isNewLine(hebStr[begin]) { /* skip leading newlines */
			begin++
		}
		for i := begin; i <= end; i++ { /* copy content */
			result[result_i] = hebStr[i]
			result_i++
		}
		for i := orinBegin; i <= end && isNewLine(hebStr[i]); i++ {
			result[result_i] = hebStr[i]
			result_i++
		}
		begin = orinBegin

		if begin <= 0 {
			if result_i < len(target) {
				result[result_i] = 0
			}
			break
		}
		begin--
		end = begin
	}
	return string(result)
}

// > func string hebrev ( string $hebrew_text [, int $max_chars_per_line = 0 ] )
func fncHebrev(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var text phpv.ZString
	var maxCharsPerLine core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &text, &maxCharsPerLine)
	if err != nil {
		return nil, err
	}

	result := hebrev(string(text), int(maxCharsPerLine.GetOrDefault(0)))
	return phpv.ZStr(string(result)), nil
}

// > func string hebrevc ( string $hebrew_text [, int $max_chars_per_line = 0 ] )
func fncHebrevc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var textArg phpv.ZString
	var maxCharsPerLine core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &textArg, &maxCharsPerLine)
	if err != nil {
		return nil, err
	}

	text := string(textArg)
	result := hebrev(text, int(maxCharsPerLine.GetOrDefault(0)))
	result = strings.ReplaceAll(result, "\n", "<br />\n")
	return phpv.ZStr(string(result)), nil
}

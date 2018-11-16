package pcre

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// partially inspired/copied from https://golang.org/src/regexp/regexp.go?s=15242:15301#L476

func pcreExpand(matches [][]byte, repl []byte) []byte {
	var res []byte
	for {
		n := bytes.IndexAny(repl, "$\\")
		if n == -1 {
			break
		}

		// append unmatched part to res
		res = append(res, repl[:n]...)
		repl = repl[n:]
		if len(repl) == 1 {
			break
		}
		name, num, rest, ok := extract(repl)
		if !ok {
			// Malformed; treat $ as raw text.
			res = append(res, repl[0])
			repl = repl[1:]
			continue
		}
		repl = rest
		if num >= 0 {
			if num < len(matches) {
				res = append(res, matches[num]...)
			}
		} else {
			// TODO named ranges?
			_ = name
		}
	}
	return append(res, repl...)
}

func extract(str []byte) (name []byte, num int, rest []byte, ok bool) {
	if len(str) < 2 || (str[0] != '$' && str[0] != '\\') {
		// definitely no match
		return
	}
	brace := false
	if str[1] == '{' {
		brace = true
		str = str[2:]
	} else {
		str = str[1:]
	}
	i := 0
	for i < len(str) {
		r, size := utf8.DecodeRune(str[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		i += size
	}
	if i == 0 {
		return
	}
	name = str[:i]
	if brace {
		if i >= len(str) || str[i] != '}' {
			return
		}
		i++
	}

	num = 0
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || '9' < name[i] || num >= 1e8 {
			num = -1
			break
		}
		num = num*10 + int(name[i]) - '0'
	}
	// Disallow leading zeros.
	if name[0] == '0' && len(name) > 1 {
		num = -1
	}

	rest = str[i:]
	ok = true
	return
}

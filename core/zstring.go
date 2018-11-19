package core

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (s ZString) ToLower() ZString {
	return ZString(strings.ToLower(string(s)))
}

func (s ZString) LooksInt() bool {
	var first bool
	if len(s) == 0 {
		return false
	}
	first = true
	for _, c := range s {
		if first && c == ' ' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		first = false
	}
	return true
}

func (s ZString) IsNumeric() bool {
	var gotDot, gotE, first bool
	if len(s) == 0 {
		return false
	}
	first = true
	for _, c := range s {
		if first && unicode.IsSpace(c) {
			continue
		}
		if first && (c == '+' || c == '-') {
			// good
			first = false
			continue
		}
		if c == '.' && !gotDot && !gotE {
			gotDot = true
			first = false
			continue // good
		}
		if c == 'e' && !gotE {
			gotE = true
			first = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		first = false
	}
	return true
}

func (z ZString) AsNumeric() (Val, error) {
	// attempt to convert z to a numeric type. First, get rid of initial spaces
	var r rune
	var l int

	for {
		if len(z) <= 1 {
			return ZInt(0), errors.New("no numeric value")
		}
		r, l = utf8.DecodeRuneInString(string(z))

		if !unicode.IsSpace(r) {
			break
		}
		z = z[l:]
	}

	p := 0
	i := 0

	for ; i < len(z); i++ {
		c := z[i]
		if c >= '0' && c <= '9' {
			if p == 0 || p == 3 {
				p += 1
			}
			continue
		}
		if c == '+' || c == '-' {
			if p == 0 || p == 3 {
				p += 1
				continue
			}
			break
		}
		if c == '.' {
			if p == 1 {
				p = 2
				continue
			}
			break
		}
		if c == 'e' || c == 'E' {
			if p < 3 {
				p = 3
				continue
			}
			break
		}
		break
	}

	if p <= 1 {
		// integer value (NB: might be too large to fit in 64 bits, in which case we'll parse as float)
		v, err := strconv.ParseInt(string(z[:i]), 10, 64)
		if err == nil {
			return ZInt(v), nil
		}
	}

	v, err := strconv.ParseFloat(string(z[:i]), 64)
	if err == nil {
		return ZFloat(v), nil
	}

	return ZInt(0), err
}

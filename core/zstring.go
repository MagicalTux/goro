package core

import "strings"

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
		if first && c == ' ' {
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

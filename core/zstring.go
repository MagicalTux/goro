package core

import "strings"

func (s ZString) ToLower() ZString {
	return ZString(strings.ToLower(string(s)))
}

package core

import "strconv"

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray struct {
	h *ZHashTable
}

// php array will use integer keys for integer values and integer-looking strings
func getKeyValue(s *ZVal) (ZInt, ZString, bool) {
	switch s.GetType() {
	case ZtInt:
		return s.v.(ZInt), "", true
	}

	str := s.String()
	if CtypeDigit(str) {
		i, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			// check if converting back results in same value
			s2 := strconv.FormatInt(i, 10)
			if str == s2 {
				// ok, we can use zint
				return ZInt(i), "", true
			}
		}
	}
	return 0, ZString(str), false
}

func (a *ZArray) GetType() ZType {
	return ZtArray
}

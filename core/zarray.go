package core

import (
	"errors"
	"strconv"
)

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray struct {
	h      *ZHashTable
	IsCopy bool // if true, write attempts will cause a copy of the object to be made (copy on write)
}

type ZArrayAccess interface {
	OffsetGet(key *ZVal) (*ZVal, error)
	OffsetSet(key, value *ZVal) error
}

// php array will use integer keys for integer values and integer-looking strings
func getArrayKeyValue(s *ZVal) (ZInt, ZString, bool) {
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

func NewZArray() *ZArray {
	return &ZArray{h: NewHashTable()}
}

func (a *ZArray) GetType() ZType {
	return ZtArray
}

func (a *ZArray) OffsetGet(key *ZVal) (*ZVal, error) {
	if key == nil || key.GetType() == ZtNull {
		return nil, errors.New("Cannot use [] for reading")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.GetInt(zi), nil
	} else {
		return a.h.GetString(zs), nil
	}
}

func (a *ZArray) OffsetSet(key, value *ZVal) error {
	if key == nil || key.GetType() == ZtNull {
		err := a.h.Append(value)
		return err
	}

	zi, zs, isint := getArrayKeyValue(key)

	var err error
	if isint {
		err = a.h.SetInt(zi, value)
	} else {
		err = a.h.SetString(zs, value)
	}

	return err
}

package phpv

import (
	"errors"
	"strconv"

	"github.com/MagicalTux/goro/core/util"
)

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray struct {
	h *ZHashTable
}

// php array will use integer keys for integer values and integer-looking strings
func getArrayKeyValue(s *ZVal) (ZInt, ZString, bool) {
	switch s.GetType() {
	case ZtNull:
		return ZInt(0), "", true
	case ZtBool:
		if s.Value().(ZBool) {
			return ZInt(1), "", true
		} else {
			return ZInt(0), "", true
		}
	case ZtInt:
		return s.Value().(ZInt), "", true
	}

	str := s.String()
	if util.CtypeDigit(str) {
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

func (a *ZArray) ZVal() *ZVal {
	return &ZVal{a}
}

func (a *ZArray) Dup() *ZArray {
	return &ZArray{h: a.h.Dup()}
}

func (a *ZArray) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtBool, ZtInt, ZtFloat:
		if a.h.count > 0 {
			return ZBool(true).AsVal(ctx, t)
		} else {
			return ZBool(false).AsVal(ctx, t)
		}
	case ZtString:
		// TODO emit warning "Array to string conversion"
		return ZString("Array"), nil
	case ZtArray:
		return a, nil
	}
	return nil, nil
}

func (a *ZArray) HasStringKeys() bool {
	return a.h.HasStringKeys()
}

func (a *ZArray) OffsetGet(ctx Context, key *ZVal) (*ZVal, error) {
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

func (a *ZArray) OffsetSet(ctx Context, key, value *ZVal) error {
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

func (a *ZArray) OffsetUnset(ctx Context, key *ZVal) error {
	if key == nil || key.GetType() == ZtNull {
		return errors.New("Cannot use [] for unset")
	}

	zi, zs, isint := getArrayKeyValue(key)
	if isint {
		return a.h.UnsetInt(zi)
	} else {
		return a.h.UnsetString(zs)
	}
}

func (a *ZArray) OffsetExists(ctx Context, key *ZVal) (bool, error) {
	if key == nil || key.GetType() == ZtNull {
		return false, errors.New("Cannot use [] for isset")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.HasInt(zi), nil
	} else {
		return a.h.HasString(zs), nil
	}
}

func (a *ZArray) NewIterator() ZIterator {
	return a.h.NewIterator()
}

func (a *ZArray) Count(ctx Context) ZInt {
	return a.h.count
}

func (a *ZArray) MergeArray(b *ZArray) error {
	// copy values from b to a
	return a.h.MergeTable(b.h)
}

func (a *ZArray) MergeTable(h *ZHashTable) error {
	// copy values from b to a
	return a.h.MergeTable(h)
}

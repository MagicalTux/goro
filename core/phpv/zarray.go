package phpv

import (
	"iter"
	"strconv"
)

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray struct {
	h *ZHashTable
}

// php array will use integer keys for integer values and integer-looking strings
func getArrayKeyValue(s Val) (ZInt, ZString, bool) {
	switch s.GetType() {
	case ZtNull:
		return ZInt(0), "", true
	case ZtBool:
		if s.Value().(ZBool) {
			return ZInt(1), "", true
		} else {
			return ZInt(0), "", true
		}
	case ZtFloat:
		n := s.Value().(ZFloat)
		return ZInt(n), "", true
	case ZtInt:
		return s.Value().(ZInt), "", true
	case ZtString:
		str := s.String()
		if ZString(str).LooksInt() {
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
	default:
		return 0, "", false
	}

}

func NewZArray() *ZArray {
	return &ZArray{h: NewHashTable()}
}

func (a *ZArray) String() string {
	return "Array"
}

func (a *ZArray) GetType() ZType {
	return ZtArray
}

func (a *ZArray) ZVal() *ZVal {
	return NewZVal(a)
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

func (a *ZArray) OffsetGet(ctx Context, key Val) (*ZVal, error) {
	if key == nil || key.GetType() == ZtNull {
		return nil, ctx.Errorf("Cannot use [] for reading")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.GetInt(zi), nil
	} else {
		return a.h.GetString(zs), nil
	}
}

func (a *ZArray) OffsetKeyAt(ctx Context, index int) (*ZVal, error) {
	i := 0
	for k := range a.Iterate(ctx) {
		if i == index {
			return k, nil
		}
		i++
	}
	return NewZVal(ZNull{}), nil
}

func (a *ZArray) OffsetAt(ctx Context, index int) (*ZVal, *ZVal, error) {
	i := 0
	for k, v := range a.Iterate(ctx) {
		if i == index {
			return k, v, nil
		}
		i++
	}
	return NewZVal(ZNull{}), NewZVal(ZNull{}), nil
}

func (a *ZArray) OffsetCheck(ctx Context, key Val) (*ZVal, bool, error) {
	if key == nil || key.GetType() == ZtNull {
		return nil, false, ctx.Errorf("Cannot use [] for reading")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.GetInt(zi), a.h.HasInt(zi), nil
	} else {
		return a.h.GetString(zs), a.h.HasString(zs), nil
	}
}

func (a *ZArray) OffsetSet(ctx Context, key Val, value *ZVal) error {
	if value == nil {
		return a.OffsetUnset(ctx, key)
	}

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

func (a *ZArray) OffsetUnset(ctx Context, key Val) error {
	if key == nil || key.GetType() == ZtNull {
		return ctx.Errorf("Cannot use [] for unset")
	}

	zi, zs, isint := getArrayKeyValue(key)
	if isint {
		return a.h.UnsetInt(zi)
	} else {
		return a.h.UnsetString(zs)
	}
}

func (a *ZArray) OffsetContains(ctx Context, val Val) (bool, error) {
	for _, v := range a.Iterate(ctx) {
		if ok, _ := Equals(ctx, val.ZVal(), v); ok {
			return true, nil
		}
	}
	return false, nil
}

func (a *ZArray) OffsetExists(ctx Context, key Val) (bool, error) {
	if key == nil || key.GetType() == ZtNull {
		return false, ctx.Errorf("Cannot use [] for isset")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.HasInt(zi), nil
	} else {
		return a.h.HasString(zs), nil
	}
}

func Every(ctx Context, array *ZArray, predicate func(*ZVal) bool) bool {
	for _, x := range array.Iterate(ctx) {
		if !predicate(x) {
			return false
		}
	}
	return true
}

func (a *ZArray) IntKeys(ctx Context) []ZInt {
	var keys []ZInt
	for key := range a.Iterate(ctx) {
		if key.GetType() == ZtInt {
			keys = append(keys, key.AsInt(ctx))
		}
	}
	return keys
}

func (a *ZArray) StringKeys(ctx Context) []ZString {
	var keys []ZString
	for key := range a.Iterate(ctx) {
		if key.GetType() == ZtString {
			keys = append(keys, key.AsString(ctx))
		}
	}
	return keys
}

func (a *ZArray) ByteArrayKeys(ctx Context) [][]byte {
	var keys [][]byte
	for key := range a.Iterate(ctx) {
		if key.GetType() == ZtString {
			keys = append(keys, []byte(key.AsString(ctx)))
		}
	}
	return keys
}

func (a *ZArray) Iterate(ctx Context) iter.Seq2[*ZVal, *ZVal] {
	return a.h.NewIterator().Iterate(ctx)
}

func (a *ZArray) Clear(ctx Context) error {
	a.h.Clear()
	return nil
}

// Similar to Clear, but still allows iteration over deleted items
func (a *ZArray) Empty(ctx Context) error {
	a.h.Empty()
	return nil
}

func (a *ZArray) NewIterator() ZIterator {
	return a.h.NewIterator()
}

func (a *ZArray) MainIterator() ZIterator {
	return a.h.mainIterator
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

func (a *ZArray) HashTable() *ZHashTable {
	return a.h
}

func (a *ZArray) Value() Val {
	return a
}

func (a *ZArray) Reset(ctx Context) {
	a.h.ResetIntKeys()
	a.h.mainIterator.Reset(ctx)
}

func (a *ZArray) Equals(ctx Context, b *ZArray) bool {
	if a.Count(ctx) != b.Count(ctx) {
		return false
	}
	for k, v1 := range a.Iterate(ctx) {
		v2, found, _ := b.OffsetCheck(ctx, k)
		if !found {
			return false
		}
		equals, _ := Equals(ctx, v1, v2)
		if !equals {
			return false
		}

	}
	return true
}

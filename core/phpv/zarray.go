package phpv

import (
	"iter"
	"strconv"

	"github.com/MagicalTux/goro/core/logopt"
)

// php arrays work with two kind of keys

// we store values in _d with a regular index

type ZArray struct {
	h *ZHashTable
}

// isNilKey checks if a Val represents "no key specified" (nil interface or nil *ZVal pointer),
// as opposed to an actual null value used as a key. In PHP, $arr[] uses nil to mean "append",
// while $arr[null] uses a ZNull-typed value to mean key "".
func isNilKey(key Val) bool {
	if key == nil {
		return true
	}
	if zv, ok := key.(*ZVal); ok && zv == nil {
		return true
	}
	return false
}

// php array will use integer keys for integer values and integer-looking strings
func getArrayKeyValue(s Val) (ZInt, ZString, bool) {
	switch s.GetType() {
	case ZtNull:
		return 0, "", false
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
	case ZtResource:
		if r, ok := s.Value().(Resource); ok {
			return ZInt(r.GetResourceID()), "", true
		}
		return 0, "", true
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

// NewZArrayTracked creates a new ZArray with memory tracking enabled.
// The tracker is notified when elements are added or removed.
func NewZArrayTracked(mt MemTracker) *ZArray {
	h := NewHashTable()
	h.SetMemTracker(mt)
	return &ZArray{h: h}
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

// DeepCopy creates an immediate independent copy without using COW.
// The original array's iterators remain stable.
func (a *ZArray) DeepCopy() *ZArray {
	return &ZArray{h: a.h.DeepCopy()}
}

// IsRecursive checks if the array contains a reference to itself (directly or indirectly).
func (a *ZArray) IsRecursive() bool {
	return a.isRecursiveWith(make(map[*ZHashTable]bool))
}

func (a *ZArray) isRecursiveWith(seen map[*ZHashTable]bool) bool {
	if seen[a.h] {
		return true
	}
	seen[a.h] = true
	for cur := a.h.first; cur != nil; cur = cur.next {
		val := cur.v
		if val == nil {
			continue
		}
		resolved := val.Nude()
		if innerArr, ok := resolved.Value().(*ZArray); ok {
			if innerArr.isRecursiveWith(seen) {
				return true
			}
		}
	}
	delete(seen, a.h)
	return false
}

// DeepCopyStripRefs creates a deep copy of the array with all references
// resolved to plain values. Used by define() to snapshot constant arrays.
func (a *ZArray) DeepCopyStripRefs(ctx Context) *ZArray {
	result := NewZArray()
	for key, val := range a.Iterate(ctx) {
		if val == nil {
			continue
		}
		// Dereference any ZVal reference chains
		resolved := val.Nude()
		// Deep copy: if the inner value is an array, recursively strip refs
		if innerArr, ok := resolved.Value().(*ZArray); ok {
			stripped := innerArr.DeepCopyStripRefs(ctx)
			result.OffsetSet(ctx, key.Value(), stripped.ZVal())
		} else {
			// Create a fresh ZVal with the resolved value (no reference)
			result.OffsetSet(ctx, key.Value(), NewZVal(resolved.Value()))
		}
	}
	return result
}

// SeparateCow forces copy-on-write separation if needed. This must be called
// before taking references to hash table entries (e.g., for by-ref spread)
// to avoid modifying data shared with other arrays.
func (a *ZArray) SeparateCow() {
	a.h.SeparateCow()
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
		if ctx != nil {
			ctx.Warn("Array to string conversion", logopt.NoFuncName(true))
		}
		return ZString("Array"), nil
	case ZtArray:
		return a, nil
	case ZtObject:
		if NewStdClassFunc == nil {
			return nil, nil
		}
		obj, err := NewStdClassFunc(ctx)
		if err != nil {
			return nil, err
		}
		// Copy array entries as object properties
		it := a.h.NewIterator()
		for {
			if !it.Valid(ctx) {
				break
			}
			key, err := it.Key(ctx)
			if err != nil {
				break
			}
			val, err := it.Current(ctx)
			if err != nil {
				break
			}
			obj.ObjectSet(ctx, key, val)
			it.Next(ctx)
		}
		return obj, nil
	}
	return nil, nil
}

func (a *ZArray) HasStringKeys() bool {
	return a.h.HasStringKeys()
}

func (a *ZArray) OffsetGet(ctx Context, key Val) (*ZVal, error) {
	if isNilKey(key) {
		return nil, ctx.Errorf("Cannot use [] for reading")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		return a.h.GetInt(zi), nil
	} else {
		return a.h.GetString(zs), nil
	}
}

// OffsetGetWarn is like OffsetGet but produces a warning for undefined keys (used by user-level array access)
func (a *ZArray) OffsetGetWarn(ctx Context, key Val) (*ZVal, error) {
	if isNilKey(key) {
		return nil, ctx.Errorf("Cannot use [] for reading")
	}

	zi, zs, isint := getArrayKeyValue(key)

	if isint {
		if !a.h.HasInt(zi) {
			ctx.Warn("Undefined array key %d", zi, logopt.NoFuncName(true))
			return ZNULL.ZVal(), nil
		}
		return a.h.GetInt(zi), nil
	}
	if !a.h.HasString(zs) {
		ctx.Warn("Undefined array key \"%s\"", zs, logopt.NoFuncName(true))
		return ZNULL.ZVal(), nil
	}
	return a.h.GetString(zs), nil
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
	if isNilKey(key) {
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
	if isNilKey(key) {
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
	if isNilKey(key) {
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
	if isNilKey(key) {
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
		// Include both string and integer keys (convert int keys to string representation)
		keys = append(keys, []byte(key.AsString(ctx)))
	}
	return keys
}

func (a *ZArray) Iterate(ctx Context) iter.Seq2[*ZVal, *ZVal] {
	return a.h.NewIterator().Iterate(ctx)
}

// IterateRaw returns an iterator that yields raw ZVals from the hash table
// without copying, preserving reference wrappers. Used by serialize() to
// detect PHP references (& references) between values.
func (a *ZArray) IterateRaw(ctx Context) iter.Seq2[*ZVal, *ZVal] {
	return a.h.NewIterator().IterateRaw(ctx)
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

// StrictEquals compares two arrays with strict comparison (===).
// Same keys in the same order, with values compared strictly.
// References are transparent (dereferenced before comparison).
func (a *ZArray) StrictEquals(ctx Context, b *ZArray) bool {
	if a.Count(ctx) != b.Count(ctx) {
		return false
	}
	// Walk both linked lists in order
	nodeA := a.h.first
	nodeB := b.h.first
	for nodeA != nil && nodeB != nil {
		// Skip deleted nodes
		if nodeA.deleted {
			nodeA = nodeA.next
			continue
		}
		if nodeB.deleted {
			nodeB = nodeB.next
			continue
		}
		// Compare keys: must be same type and value
		kA := nodeA.k
		kB := nodeB.k
		if kA.GetType() != kB.GetType() {
			return false
		}
		switch kA.GetType() {
		case ZtInt:
			if kA.(ZInt) != kB.(ZInt) {
				return false
			}
		case ZtString:
			if kA.(ZString) != kB.(ZString) {
				return false
			}
		default:
			return false
		}
		// Compare values strictly (dereferences references)
		eq, _ := StrictEquals(ctx, nodeA.v, nodeB.v)
		if !eq {
			return false
		}
		nodeA = nodeA.next
		nodeB = nodeB.next
	}
	// Skip remaining deleted nodes
	for nodeA != nil && nodeA.deleted {
		nodeA = nodeA.next
	}
	for nodeB != nil && nodeB.deleted {
		nodeB = nodeB.next
	}
	return nodeA == nil && nodeB == nil
}

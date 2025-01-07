package phpv

import (
	"sync"
)

type hashTableVal struct {
	prev, next *hashTableVal
	k          Val
	v          *ZVal
	deleted    bool

	// ommitedKey is set to true when key is ommited.
	// For instance, [1 => 'a', 'b', 9 => 'c'], 'b' here has an ommited key.
	// Added specifically for array_flip because it needs to distinguish
	// array entries with omitted keys. Not sure if it's useful for
	// anything else.
	omittedKey bool
}

type ZHashTable struct {
	first, last *hashTableVal
	lock        sync.RWMutex
	inc         ZInt
	count       ZInt
	cow         bool

	_idx_s map[ZString]*hashTableVal
	_idx_i map[ZInt]*hashTableVal

	mainIterator *zhashtableIterator
}

func NewHashTable() *ZHashTable {
	n := &ZHashTable{
		_idx_s: make(map[ZString]*hashTableVal),
		_idx_i: make(map[ZInt]*hashTableVal),
	}
	n.mainIterator = &zhashtableIterator{n, nil}
	return n
}

func (z *ZHashTable) Dup() *ZHashTable {
	z.lock.Lock()
	defer z.lock.Unlock()

	// setting z.cow prevents *all* writes on this array
	z.cow = true

	// do not blindly copy all of z as it includes the lock
	n := &ZHashTable{
		first:  z.first,
		last:   z.last,
		inc:    z.inc,
		count:  z.count,
		cow:    true,
		_idx_s: z._idx_s,
		_idx_i: z._idx_i,
	}

	cur := z.mainIterator.cur
	if cur == nil {
		cur = z.first
	}
	n.mainIterator = &zhashtableIterator{n, cur}

	return n
}

func (z *ZHashTable) Clear() {
	z.lock.Lock()
	defer z.lock.Unlock()

	for _, v := range z._idx_i {
		v.deleted = true
	}
	for _, v := range z._idx_s {
		v.deleted = true
	}
	z.count = 0
	z.inc = 0
	z.first = nil
	z.last = nil
	z.mainIterator.cur = nil

	clear(z._idx_i)
	clear(z._idx_s)
}

// Similar to Clear, but doesn't set the deleted flag
func (z *ZHashTable) Empty() {
	z.lock.Lock()
	defer z.lock.Unlock()

	z.count = 0
	z.inc = 0
	z.first = nil
	z.last = nil
	z.mainIterator.cur = nil

	clear(z._idx_i)
	clear(z._idx_s)
}

func (z *ZHashTable) doCopy() error {
	// called after z.lock has been locked and when z.cow is true
	// this will copy all the elements from the array and return a new, modifiable array (and also re-generate both indexes)
	var nc, first *hashTableVal
	_idx_s := make(map[ZString]*hashTableVal)
	_idx_i := make(map[ZInt]*hashTableVal)

	for c := z.first; c != nil; c = c.next {
		if c.deleted {
			if z.mainIterator.cur == c {
				z.mainIterator.cur = nil
			}
			continue
		}
		nc = &hashTableVal{
			k:          c.k,
			v:          c.v.ZVal(),
			omittedKey: c.omittedKey,
			prev:       nc,
		}

		if z.mainIterator.cur == c {
			z.mainIterator.cur = nc
		}

		if first == nil {
			first = nc
		} else {
			nc.prev.next = nc
		}

		switch k := nc.k.(type) {
		case ZInt:
			_idx_i[k] = nc
		case ZString:
			_idx_s[k] = nc
		default:
			// shouldn't happen
			panic("invalid index type in array")
		}
	}

	// ok, regen is done, set values
	z.first = first
	z.last = nc
	z._idx_s = _idx_s
	z._idx_i = _idx_i
	z.cow = false
	return nil
}

func (z *ZHashTable) NewIterator() ZIterator {
	return &zhashtableIterator{z, z.first}
}

func (z *ZHashTable) GetString(k ZString) *ZVal {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_s[k]
	if !ok {
		return NewZVal(ZNull{})
	}
	return t.v
}

func (z *ZHashTable) GetStringB(k ZString) (*ZVal, bool) {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_s[k]
	if !ok {
		return NewZVal(ZNull{}), false
	}
	return t.v, true
}

func (z *ZHashTable) HasString(k ZString) bool {
	z.lock.RLock()
	defer z.lock.RUnlock()

	_, ok := z._idx_s[k]
	return ok
}

func (z *ZHashTable) SetString(k ZString, v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	t, ok := z._idx_s[k]
	if ok {
		t.v.Set(v)
		return nil
	}
	// append
	nt := &hashTableVal{k: k, v: v}
	z.count += 1
	z._idx_s[k] = nt
	if z.last == nil {
		z.first = nt
		z.last = nt
		return nil
	}
	z.last.next = nt
	nt.prev = z.last
	z.last = nt
	return nil
}

func (z *ZHashTable) UnsetString(k ZString) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	t, ok := z._idx_s[k]
	if !ok {
		return nil
	}
	// remove
	z.count -= 1
	delete(z._idx_s, k)
	t.deleted = true

	if z.first == t {
		z.first = t.next
	}
	if z.last == t {
		z.last = t.prev
	}
	if t.prev != nil {
		t.prev.next = t.next
	}
	if t.next != nil {
		t.next.prev = t.prev
	}
	return nil
}

func (z *ZHashTable) GetInt(k ZInt) *ZVal {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_i[k]
	if !ok {
		return NewZVal(ZNull{})
	}
	return t.v
}

func (z *ZHashTable) SetInt(k ZInt, v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	t, ok := z._idx_i[k]
	if ok {
		t.v.Set(v)
		return nil
	}
	// append
	nt := &hashTableVal{k: k, v: v}
	z.count += 1
	z._idx_i[k] = nt
	if z.last == nil {
		z.first = nt
		z.last = nt
		return nil
	}
	z.last.next = nt
	nt.prev = z.last
	z.last = nt
	return nil
}

func (z *ZHashTable) UnsetInt(k ZInt) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	t, ok := z._idx_i[k]
	if !ok {
		return nil
	}
	// remove
	z.count -= 1
	delete(z._idx_i, k)
	t.deleted = true

	if z.first == t {
		z.first = t.next
	}
	if z.last == t {
		z.last = t.prev
	}
	if t.prev != nil {
		t.prev.next = t.next
	}
	if t.next != nil {
		t.next.prev = t.prev
	}
	return nil
}

func (z *ZHashTable) HasInt(k ZInt) bool {
	z.lock.RLock()
	defer z.lock.RUnlock()

	_, ok := z._idx_i[k]
	return ok
}

func (z *ZHashTable) Append(v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	for {
		if _, ok := z._idx_i[z.inc]; ok {
			z.inc += 1
		} else {
			break
		}
	}

	nt := &hashTableVal{k: z.inc, v: v, omittedKey: true}
	z._idx_i[z.inc] = nt
	z.inc += 1
	z.count += 1

	if z.last == nil {
		z.first = nt
		z.last = nt
		return nil
	}
	z.last.next = nt
	nt.prev = z.last
	z.last = nt
	return nil
}

func (z *ZHashTable) MergeTable(b *ZHashTable) error {
	// merge values from b into z
	b.lock.RLock()
	defer b.lock.RUnlock()
	z.lock.Lock()
	defer z.lock.Unlock()

	if z.cow {
		z.doCopy()
	}

	for c := b.first; c != nil; c = c.next {
		if c.deleted {
			continue
		}
		nc := &hashTableVal{
			prev:       z.last,
			k:          c.k,
			v:          c.v.ZVal(),
			omittedKey: c.omittedKey,
		}
		// index value
		switch k := nc.k.(type) {
		case ZInt:
			// create new value
			nc.k = z.inc
			z._idx_i[z.inc] = nc
			z.inc += 1
			z.count += 1
		case ZString:
			// check if existing
			e, found := z._idx_s[k]
			if found {
				// ok, just set value in existing
				e.v = nc.v
				nc = nil
			} else {
				z.count += 1
			}
		}
		if nc == nil {
			continue
		}
		if z.last == nil {
			// empty array
			z.first = nc
			z.last = nc
			continue
		}
		nc.prev.next = nc
		z.last = nc
	}
	return nil
}

func (z ZHashTable) HasStringKeys() bool {
	return len(z._idx_s) > 0
}

func (z *ZHashTable) Array() *ZArray {
	return &ZArray{h: z}
}

func (z *ZHashTable) Count() ZInt {
	return z.count
}

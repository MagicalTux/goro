package core

type hashTableVal struct {
	prev, next *hashTableVal
	v          *ZVal
}

type ZHashTable struct {
	first, last *hashTableVal

	_idx_s map[ZString]*hashTableVal
	_idx_i map[ZInt]*hashTableVal
}

func NewHashTable() *ZHashTable {
	return &ZHashTable{
		_idx_s: make(map[ZString]*hashTableVal),
		_idx_i: make(map[ZInt]*hashTableVal),
	}
}

func (z *ZHashTable) GetString(k string) *ZVal {
	t, ok := z._idx_s[ZString(k)]
	if !ok {
		return nil
	}
	return t.v
}

func (z *ZHashTable) SetString(k string, v *ZVal) error {
	t, ok := z._idx_s[ZString(k)]
	if ok {
		t.v = v
		return nil
	}
	// append
	nt := &hashTableVal{v: v}
	z._idx_s[ZString(k)] = nt
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

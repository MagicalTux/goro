package core

import "sync"

type hashTableVal struct {
	prev, next *hashTableVal
	v          *ZVal
}

type ZHashTable struct {
	first, last *hashTableVal
	lock        sync.RWMutex
	inc         ZInt

	_idx_s map[ZString]*hashTableVal
	_idx_i map[ZInt]*hashTableVal
}

func NewHashTable() *ZHashTable {
	return &ZHashTable{
		_idx_s: make(map[ZString]*hashTableVal),
		_idx_i: make(map[ZInt]*hashTableVal),
	}
}

func (z *ZHashTable) GetString(k ZString) *ZVal {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_s[k]
	if !ok {
		return nil
	}
	return t.v
}

func (z *ZHashTable) SetString(k ZString, v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	t, ok := z._idx_s[k]
	if ok {
		t.v = v
		return nil
	}
	// append
	nt := &hashTableVal{v: v}
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

func (z *ZHashTable) SetInt(k ZInt, v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	t, ok := z._idx_i[k]
	if ok {
		t.v = v
		return nil
	}
	// append
	nt := &hashTableVal{v: v}
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

package core

import "sync"

type hashTableVal struct {
	prev, next *hashTableVal
	k          Val
	v          *ZVal
	deleted    bool
}

type ZHashTable struct {
	first, last *hashTableVal
	lock        sync.RWMutex
	inc         ZInt
	count       ZInt

	_idx_s map[ZString]*hashTableVal
	_idx_i map[ZInt]*hashTableVal
}

func NewHashTable() *ZHashTable {
	return &ZHashTable{
		_idx_s: make(map[ZString]*hashTableVal),
		_idx_i: make(map[ZInt]*hashTableVal),
	}
}

func (z *ZHashTable) NewIterator() ZIterator {
	return &zhashtableIterator{z, z.first}
}

func (z *ZHashTable) GetString(k ZString) *ZVal {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_s[k]
	if !ok {
		return &ZVal{ZNull{}}
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

func (z *ZHashTable) GetInt(k ZInt) *ZVal {
	z.lock.RLock()
	defer z.lock.RUnlock()

	t, ok := z._idx_i[k]
	if !ok {
		return &ZVal{ZNull{}}
	}
	return t.v
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

func (z *ZHashTable) Append(v *ZVal) error {
	z.lock.Lock()
	defer z.lock.Unlock()

	for {
		if _, ok := z._idx_i[z.inc]; ok {
			z.inc += 1
		} else {
			break
		}
	}

	nt := &hashTableVal{k: z.inc, v: v}
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

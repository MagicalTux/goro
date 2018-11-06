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
	t := z._idx_s[ZString(k)]
	if t == nil {
		return nil
	}
	return t.v
}

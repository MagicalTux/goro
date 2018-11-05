package core

type hashTableVal struct {
	prev, next *hashTableVal
	v          ZVal
}

type ZHashTable struct {
	first, last *hashTableVal

	_idx_s map[ZString]*hashTableVal
	_idx_i map[ZInt]*hashTableVal
}

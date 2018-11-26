package core

import (
	"fmt"
	"sync"
)

type MemMgr struct {
	limit uint64
	cur   uint64
	l     sync.Mutex
}

func NewMemMgr(limit uint64) *MemMgr {
	return &MemMgr{limit: limit}
}

func (m *MemMgr) Alloc(ctx Context, s uint64) error {
	m.l.Lock()
	defer m.l.Unlock()

	if m.limit == 0 {
		// no limit
		m.cur = m.cur + s // we don't check for overflow
		return nil
	}

	if m.cur >= m.limit {
		return fmt.Errorf("Out of memory (currently allocated %d) (tried to allocate additional %d bytes)", m.cur, s)
	}

	if m.limit-m.cur < s {
		return fmt.Errorf("Out of memory (currently allocated %d) (tried to allocate additional %d bytes)", m.cur, s)
	}

	// because s is below difference between m.limit and m.cur there won't be any overflow
	m.cur += s

	return nil
}

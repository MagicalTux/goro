package phpctx

import (
	"fmt"
	"io"
	"sync"

	"github.com/MagicalTux/goro/core/phpv"
)

type MemMgr struct {
	limit uint64
	cur   uint64
	l     sync.Mutex
}

func NewMemMgr(limit uint64) *MemMgr {
	return &MemMgr{limit: limit}
}

func (m *MemMgr) Alloc(ctx phpv.Context, s uint64) error {
	m.l.Lock()
	defer m.l.Unlock()

	return m.internalAlloc(s)
}

func (m *MemMgr) internalAlloc(s uint64) error {
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

func (m *MemMgr) Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	// io.Copy but with memory limit checks
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if em := m.internalAlloc(uint64(nw)); em != nil {
				err = em
				break
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return
}

package phpctx

import (
	"fmt"
	"io"
	"sync/atomic"

	"github.com/MagicalTux/goro/core/phpv"
)

type MemMgr struct {
	limit   int64 // -1 = unlimited, 0 = unlimited, >0 = limit in bytes
	tracked int64 // current tracked PHP allocations (atomic)
	peak    int64 // peak tracked (atomic)
}

func NewMemMgr(limit int64) *MemMgr {
	return &MemMgr{limit: limit}
}

// SetLimit updates the memory limit. A limit of 0 or -1 means unlimited.
func (m *MemMgr) SetLimit(limit int64) {
	atomic.StoreInt64(&m.limit, limit)
}

// Limit returns the current memory limit in bytes (0 = unlimited).
func (m *MemMgr) Limit() int64 {
	return atomic.LoadInt64(&m.limit)
}

// Alloc tracks a new PHP-level allocation of size bytes.
// Returns a fatal PhpError if the allocation would exceed the memory limit.
func (m *MemMgr) Alloc(size int64) error {
	newTracked := atomic.AddInt64(&m.tracked, size)

	// Update peak
	for {
		oldPeak := atomic.LoadInt64(&m.peak)
		if newTracked <= oldPeak {
			break
		}
		if atomic.CompareAndSwapInt64(&m.peak, oldPeak, newTracked) {
			break
		}
	}

	// Check limit
	limit := atomic.LoadInt64(&m.limit)
	if limit <= 0 {
		return nil // unlimited
	}
	if newTracked > limit {
		// Revert the allocation so the counter stays accurate
		atomic.AddInt64(&m.tracked, -size)
		return &phpv.PhpError{
			Err:  fmt.Errorf("Allowed memory size of %d bytes exhausted (tried to allocate %d bytes)", limit, size),
			Code: phpv.E_ERROR,
		}
	}
	return nil
}

// Free releases a previously tracked allocation.
func (m *MemMgr) Free(size int64) {
	atomic.AddInt64(&m.tracked, -size)
}

// Usage returns the current tracked memory usage in bytes.
func (m *MemMgr) Usage() int64 {
	v := atomic.LoadInt64(&m.tracked)
	if v < 0 {
		return 0
	}
	return v
}

// PeakUsage returns the peak tracked memory usage in bytes.
func (m *MemMgr) PeakUsage() int64 {
	return atomic.LoadInt64(&m.peak)
}

// ResetPeak sets peak = current tracked usage.
func (m *MemMgr) ResetPeak() {
	cur := atomic.LoadInt64(&m.tracked)
	atomic.StoreInt64(&m.peak, cur)
}

// MemAlloc implements phpv.MemTracker interface.
func (m *MemMgr) MemAlloc(size int64) error { return m.Alloc(size) }

// MemFree implements phpv.MemTracker interface.
func (m *MemMgr) MemFree(size int64) { m.Free(size) }

// Copy copies data from src to dst while tracking memory usage.
func (m *MemMgr) Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if em := m.Alloc(int64(nw)); em != nil {
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

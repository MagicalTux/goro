package phpctx

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/MagicalTux/goro/core/phpv"
)

// sharedMemAlloc is a process-wide snapshot of runtime.MemStats.Alloc,
// updated periodically by a background goroutine. This avoids the
// stop-the-world cost of calling runtime.ReadMemStats in every Tick().
var sharedMemAlloc uint64

func init() {
	// Start a background goroutine that periodically snapshots memory usage.
	go func() {
		var ms runtime.MemStats
		for {
			func() {
				defer func() { recover() }()
				runtime.ReadMemStats(&ms)
				atomic.StoreUint64(&sharedMemAlloc, ms.Alloc)
			}()
			time.Sleep(50 * time.Millisecond)
		}
	}()
}

// InitBaselineMemory records the current memory usage as the baseline
// for this Global context, so Tick() can measure per-script usage.
func (g *Global) InitBaselineMemory() {
	defer func() { recover() }()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	g.baselineAlloc = ms.Alloc
	g.lastMemCheck = ms.Alloc
}

// checkMemoryLimit checks PHP-level tracked memory and also uses the
// globally shared runtime snapshot as a safety net for untracked allocations.
func (g *Global) checkMemoryLimit() error {
	limit := g.mem.Limit()
	if limit <= 0 {
		return nil // unlimited
	}

	// Check PHP-level tracked allocations
	tracked := g.mem.Usage()
	if tracked > limit {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Allowed memory size of %d bytes exhausted (tried to allocate %d bytes)", limit, tracked-limit),
			Code: phpv.E_ERROR,
			Loc:  g.l,
		}
	}

	// Also check runtime memory as a safety net for untracked allocations
	currentAlloc := atomic.LoadUint64(&sharedMemAlloc)
	if currentAlloc == 0 {
		return nil // background goroutine hasn't run yet, skip runtime check
	}

	var usage uint64
	if currentAlloc > g.baselineAlloc {
		usage = currentAlloc - g.baselineAlloc
	}
	g.lastMemCheck = currentAlloc

	if usage > uint64(limit) {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Allowed memory size of %d bytes exhausted (tried to allocate %d bytes)", limit, usage-uint64(limit)),
			Code: phpv.E_ERROR,
			Loc:  g.l,
		}
	}
	return nil
}

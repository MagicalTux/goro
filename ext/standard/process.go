package standard

import (
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int|false getmypid ( void )
func fncGetmypid(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(os.Getpid()).ZVal(), nil
}

// > func int|false getmyuid ( void )
func fncGetmyuid(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(os.Getuid()).ZVal(), nil
}

// > func int memory_get_usage ([ bool $real_usage = false ] )
func fncMemoryGetUsage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var realUsage *bool
	_, err := core.Expand(ctx, args, &realUsage)
	if err != nil {
		return nil, err
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if realUsage != nil && *realUsage {
		return phpv.ZInt(m.HeapSys).ZVal(), nil
	}
	return phpv.ZInt(m.HeapAlloc).ZVal(), nil
}

// > func int memory_get_peak_usage ([ bool $real_usage = false ] )
func fncMemoryGetPeakUsage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var realUsage *bool
	_, err := core.Expand(ctx, args, &realUsage)
	if err != nil {
		return nil, err
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if realUsage != nil && *realUsage {
		// Go doesn't track peak HeapSys separately; use HeapSys as approximation
		return phpv.ZInt(m.HeapSys).ZVal(), nil
	}
	// TotalAlloc is cumulative bytes allocated (never decreases), serves as
	// an approximation of peak usage since Go doesn't track peak HeapAlloc.
	return phpv.ZInt(m.TotalAlloc).ZVal(), nil
}

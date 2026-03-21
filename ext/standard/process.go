package standard

import (
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
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

	if realUsage != nil && *realUsage {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return phpv.ZInt(m.HeapSys).ZVal(), nil
	}

	// Return PHP-level tracked memory usage
	if g, ok := ctx.Global().(*phpctx.Global); ok {
		return phpv.ZInt(g.MemUsage()).ZVal(), nil
	}

	// Fallback to runtime stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return phpv.ZInt(m.HeapAlloc).ZVal(), nil
}

// > func void memory_reset_peak_usage ( void )
func fncMemoryResetPeakUsage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if g, ok := ctx.Global().(*phpctx.Global); ok {
		g.MemResetPeak()
	}
	return nil, nil
}

// > func int memory_get_peak_usage ([ bool $real_usage = false ] )
func fncMemoryGetPeakUsage(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var realUsage *bool
	_, err := core.Expand(ctx, args, &realUsage)
	if err != nil {
		return nil, err
	}

	if realUsage != nil && *realUsage {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return phpv.ZInt(m.HeapSys).ZVal(), nil
	}

	// Return PHP-level tracked peak memory usage
	if g, ok := ctx.Global().(*phpctx.Global); ok {
		return phpv.ZInt(g.MemPeakUsage()).ZVal(), nil
	}

	// Fallback to runtime stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return phpv.ZInt(m.TotalAlloc).ZVal(), nil
}

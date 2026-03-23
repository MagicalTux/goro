package core

import "github.com/MagicalTux/goro/core/phpv"

// > func int gc_collect_cycles ( void )
func stdFuncGcCollectCycles(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// runtime.GC()
	return phpv.ZInt(0).ZVal(), nil
}

// > func void gc_disable ( void )
func stdFuncGcDisable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, nil
}

// > func void gc_enable ( void )
func stdFuncGcEnable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, nil
}

// > func bool gc_enabled ( void )
func stdFuncGcEnabled(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(true).ZVal(), nil
}

// > func int gc_mem_caches ( void )
func stdFuncGcMemCaches(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(0).ZVal(), nil
}

// > func array gc_status ( void )
// Returns an array with GC information. Since Go uses a different GC model,
// we return stub values that match PHP's expected keys.
func stdFuncGcStatus(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("runs").ZVal(), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("collected").ZVal(), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("threshold").ZVal(), phpv.ZInt(10000).ZVal())
	result.OffsetSet(ctx, phpv.ZString("roots").ZVal(), phpv.ZInt(0).ZVal())
	// PHP 8.3+ added these fields
	result.OffsetSet(ctx, phpv.ZString("running").ZVal(), phpv.ZBool(false).ZVal())
	result.OffsetSet(ctx, phpv.ZString("protected").ZVal(), phpv.ZBool(false).ZVal())
	result.OffsetSet(ctx, phpv.ZString("full").ZVal(), phpv.ZBool(false).ZVal())
	result.OffsetSet(ctx, phpv.ZString("buffer_size").ZVal(), phpv.ZInt(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("application_time").ZVal(), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("collector_time").ZVal(), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("destructor_time").ZVal(), phpv.ZFloat(0).ZVal())
	result.OffsetSet(ctx, phpv.ZString("free_time").ZVal(), phpv.ZFloat(0).ZVal())
	return result.ZVal(), nil
}

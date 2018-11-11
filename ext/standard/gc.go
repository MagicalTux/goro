package standard

import "git.atonline.com/tristantech/gophp/core"

//> func int gc_collect_cycles ( void )
func stdFuncGcCollectCycles(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	// runtime.GC()
	return core.ZInt(0).ZVal(), nil
}

//> func void gc_disable ( void )
func stdFuncGcDisable(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return nil, nil
}

//> func void gc_enable ( void )
func stdFuncGcEnable(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return nil, nil
}

//> func bool gc_enabled ( void )
func stdFuncGcEnabled(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZBool(true).ZVal(), nil
}

//> func int gc_mem_caches ( void )
func stdFuncGcMemCaches(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZInt(0).ZVal(), nil
}

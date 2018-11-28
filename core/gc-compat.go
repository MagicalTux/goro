package core

import "github.com/MagicalTux/goro/core/phpv"

//> func int gc_collect_cycles ( void )
func stdFuncGcCollectCycles(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// runtime.GC()
	return phpv.ZInt(0).ZVal(), nil
}

//> func void gc_disable ( void )
func stdFuncGcDisable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, nil
}

//> func void gc_enable ( void )
func stdFuncGcEnable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, nil
}

//> func bool gc_enabled ( void )
func stdFuncGcEnabled(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(true).ZVal(), nil
}

//> func int gc_mem_caches ( void )
func stdFuncGcMemCaches(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(0).ZVal(), nil
}

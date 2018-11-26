package core

//> func int gc_collect_cycles ( void )
func stdFuncGcCollectCycles(ctx Context, args []*ZVal) (*ZVal, error) {
	// runtime.GC()
	return ZInt(0).ZVal(), nil
}

//> func void gc_disable ( void )
func stdFuncGcDisable(ctx Context, args []*ZVal) (*ZVal, error) {
	return nil, nil
}

//> func void gc_enable ( void )
func stdFuncGcEnable(ctx Context, args []*ZVal) (*ZVal, error) {
	return nil, nil
}

//> func bool gc_enabled ( void )
func stdFuncGcEnabled(ctx Context, args []*ZVal) (*ZVal, error) {
	return ZBool(true).ZVal(), nil
}

//> func int gc_mem_caches ( void )
func stdFuncGcMemCaches(ctx Context, args []*ZVal) (*ZVal, error) {
	return ZInt(0).ZVal(), nil
}

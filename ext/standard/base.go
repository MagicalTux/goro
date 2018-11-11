package standard

import "git.atonline.com/tristantech/gophp/core"

//> const PHP_VERSION: core.ZString(core.VERSION) // version of PHP

//> func string phpversion ([ string $extension ] )
func stdFuncPhpVersion(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZString(core.VERSION).ZVal(), nil
}

package standard

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core"
)

//> const PHP_VERSION: core.ZString(core.VERSION) // version of PHP

//> func bool dl ( string $library )
func stdFuncDl(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return nil, errors.New("Dynamically loaded extensions aren't enabled")
}

//> func string phpversion ([ string $extension ] )
func stdFuncPhpVersion(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZString(core.VERSION).ZVal(), nil
}

package core

import (
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// php targetted version
const VERSION = "7.3.0"

//> const PHP_VERSION: phpv.ZString(VERSION) // version of PHP
//> const PHP_MAJOR_VERSION: phpv.ZInt(7)
//> const PHP_MINOR_VERSION: phpv.ZInt(3)
//> const PHP_RELEASE_VERSION: phpv.ZInt(0)
//> const PHP_EXTRA_VERSION: phpv.ZString("")
//> const PHP_VERSION_ID: phpv.ZInt(70300)

// > func string phpversion ([ string $extension ] )
func stdFuncPhpVersion(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ext *string
	_, err := Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext != nil {
		e := phpctx.GetExt(*ext)
		if e == nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		return phpv.ZString(e.Version).ZVal(), nil
	}

	return phpv.ZString(VERSION).ZVal(), nil
}

// > func string zend_version ( void )
func stdFuncZendVersion(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString("3.2.0").ZVal(), nil
}

package core

// php targetted version
const VERSION = "7.3.0"

//> const PHP_VERSION: ZString(VERSION) // version of PHP
//> const PHP_MAJOR_VERSION: ZInt(7)
//> const PHP_MINOR_VERSION: ZInt(3)
//> const PHP_RELEASE_VERSION: ZInt(0)
//> const PHP_EXTRA_VERSION: ZString("")
//> const PHP_VERSION_ID: ZInt(70300)

//> func string phpversion ([ string $extension ] )
func stdFuncPhpVersion(ctx Context, args []*ZVal) (*ZVal, error) {
	var ext *string
	_, err := Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext != nil {
		e := GetExt(*ext)
		if e == nil {
			return ZBool(false).ZVal(), nil
		}
		return ZString(e.Version).ZVal(), nil
	}

	return ZString(VERSION).ZVal(), nil
}

//> func string zend_version ( void )
func stdFuncZendVersion(ctx Context, args []*ZVal) (*ZVal, error) {
	return ZString("3.2.0").ZVal(), nil
}

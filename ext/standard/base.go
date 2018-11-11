package standard

import (
	"errors"

	"github.com/MagicalTux/gophp/core"
)

//> const PHP_VERSION: core.ZString(core.VERSION) // version of PHP

//> func bool dl ( string $library )
func stdFuncDl(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return nil, errors.New("Dynamically loaded extensions aren't enabled")
}

//> func bool extension_loaded ( string $name )
func stdFunc(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var name string
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}
	return core.ZBool(core.HasExt(name)).ZVal(), nil
}

//> func string phpversion ([ string $extension ] )
func stdFuncPhpVersion(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var ext *string
	_, err := core.Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext != nil {
		e := core.GetExt(*ext)
		if e == nil {
			return core.ZBool(false).ZVal(), nil
		}
		return core.ZString(e.Version).ZVal(), nil
	}

	return core.ZString(core.VERSION).ZVal(), nil
}

//> func string zend_version ( void )
func stdFuncZendVersion(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZString("3.2.0").ZVal(), nil
}

//> func bool function_exists ( string $function_name )
func stdFuncFuncExists(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var fname core.ZString
	_, err := core.Expand(ctx, args, &fname)
	if err != nil {
		return nil, err
	}

	f, _ := ctx.GetGlobal().GetFunction(fname)
	return core.ZBool(f != nil).ZVal(), nil
}

//> func mixed get_cfg_var ( string $option )
func stdFuncGetCfgVar(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v core.ZString
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}
	return ctx.GetGlobal().GetConfig(v, core.ZNull{}.ZVal()), nil
}

package standard

import (
	"errors"

	"github.com/MagicalTux/goro/core"
)

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

//> func bool function_exists ( string $function_name )
func stdFuncFuncExists(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var fname core.ZString
	_, err := core.Expand(ctx, args, &fname)
	if err != nil {
		return nil, err
	}

	f, _ := ctx.Global().GetFunction(ctx, fname)
	return core.ZBool(f != nil).ZVal(), nil
}

//> func mixed get_cfg_var ( string $option )
func stdFuncGetCfgVar(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v core.ZString
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}
	return ctx.Global().GetConfig(v, core.ZNull{}.ZVal()), nil
}

//> func string php_sapi_name ( void )
func stdFuncSapiName(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return ctx.Global().GetConstant("PHP_SAPI")
}

//> func string gettype ( mixed $var )
func fncGettype(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	t := v.GetType()
	return core.ZString(t.String()).ZVal(), nil
}

//> func void flush ( void )
func fncFlush(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	ctx.Global().Flush()
	return core.ZNULL.ZVal(), nil
}

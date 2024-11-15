package standard

import (
	"errors"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool dl ( string $library )
func stdFuncDl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return nil, errors.New("Dynamically loaded extensions aren't enabled")
}

// > func bool extension_loaded ( string $name )
func stdFunc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name string
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(phpctx.HasExt(name)).ZVal(), nil
}

// > func bool function_exists ( string $function_name )
func stdFuncFuncExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fname phpv.ZString
	_, err := core.Expand(ctx, args, &fname)
	if err != nil {
		return nil, err
	}

	f, _ := ctx.Global().GetFunction(ctx, fname)
	return phpv.ZBool(f != nil).ZVal(), nil
}

// > func bool method_exists (  mixed $object , string $method_name )
func stdFuncMethodExists(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var objectArg *phpv.ZVal
	var methodName phpv.ZString
	_, err := core.Expand(ctx, args, &objectArg, &methodName)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass
	switch objectArg.GetType() {
	case phpv.ZtString:
		className := objectArg.AsString(ctx)
		class, err = ctx.Global().GetClass(ctx, className, false)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
	case phpv.ZtObject:
		obj := objectArg.AsObject(ctx)
		class = obj.GetClass()
	default:
		return nil, errors.New("Argument #1 ($object_or_class) must be of type object|string")
	}
	_, ok := class.GetMethod(methodName)

	return phpv.ZBool(ok).ZVal(), nil
}

// > func mixed get_cfg_var ( string $option )
func stdFuncGetCfgVar(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v phpv.ZString
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}
	return ctx.Global().GetConfig(v, phpv.ZNull{}.ZVal()), nil
}

// > func string php_sapi_name ( void )
func stdFuncSapiName(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	v, ok := ctx.Global().ConstantGet("PHP_SAPI")
	if !ok {
		return phpv.ZString("php").ZVal(), nil
	}
	return v.ZVal(), nil
}

// > func string gettype ( mixed $var )
func fncGettype(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZVal
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	t := v.GetType()
	return phpv.ZString(t.String()).ZVal(), nil
}

// > func void flush ( void )
func fncFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Global().Flush()
	return phpv.ZNULL.ZVal(), nil
}

package standard

import "github.com/MagicalTux/gophp/core"

//> func bool is_array ( mixed $var )
func fncIsArray(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtArray).ZVal(), nil
}

//> func bool is_bool ( mixed $var )
func fncIsBool(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtBool).ZVal(), nil
}

//> func bool is_double ( mixed $var )
func fncIsDouble(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return fncIsFloat(ctx, args)
}

//> func bool is_float ( mixed $var )
func fncIsFloat(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtFloat).ZVal(), nil
}

//> func bool is_int ( mixed $var )
func fncIsInt(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtInt).ZVal(), nil
}

//> func bool is_integer ( mixed $var )
func fncIsInteger(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return fncIsInt(ctx, args)
}

//> func bool is_long ( mixed $var )
func fncIsLong(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return fncIsInt(ctx, args)
}

//> func bool is_null ( mixed $var )
func fncIsNull(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtNull).ZVal(), nil
}

//> func bool is_numeric ( mixed $var )
func fncIsNumeric(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	switch z.Value().(type) {
	case core.ZInt, core.ZFloat:
		return core.ZBool(true).ZVal(), nil
	}

	s := z.AsString(ctx)
	return core.ZBool(s.IsNumeric()).ZVal(), nil
}

//> func bool is_object ( mixed $var )
func fncIsObject(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtObject).ZVal(), nil
}

//> func bool is_real ( mixed $var )
func fncIsReal(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return fncIsFloat(ctx, args)
}

//> func bool is_resource ( mixed $var )
func fncIsResource(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtResource).ZVal(), nil
}

//> func bool is_scalar ( mixed $var )
func fncIsScalar(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	switch z.GetType() {
	case core.ZtInt, core.ZtFloat, core.ZtString, core.ZtBool:
		return core.ZBool(true).ZVal(), nil
	}
	return core.ZBool(false).ZVal(), nil
}

//> func bool is_string ( mixed $var )
func fncIsString(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return core.ZBool(z.GetType() == core.ZtString).ZVal(), nil
}

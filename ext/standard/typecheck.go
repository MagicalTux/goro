package standard

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func bool is_array ( mixed $var )
func fncIsArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtArray).ZVal(), nil
}

// > func bool is_bool ( mixed $var )
func fncIsBool(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtBool).ZVal(), nil
}

// > func bool is_double ( mixed $var )
func fncIsDouble(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncIsFloat(ctx, args)
}

// > func bool is_float ( mixed $var )
func fncIsFloat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtFloat).ZVal(), nil
}

// > func bool is_int ( mixed $var )
func fncIsInt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtInt).ZVal(), nil
}

// > func bool is_integer ( mixed $var )
func fncIsInteger(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncIsInt(ctx, args)
}

// > func bool is_long ( mixed $var )
func fncIsLong(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncIsInt(ctx, args)
}

// > func bool is_null ( mixed $var )
func fncIsNull(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtNull).ZVal(), nil
}

// > func bool is_numeric ( mixed $var )
func fncIsNumeric(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	switch z.Value().(type) {
	case phpv.ZInt, phpv.ZFloat:
		return phpv.ZBool(true).ZVal(), nil
	case phpv.ZBool, phpv.ZNull:
		// Booleans and null are not considered numeric by is_numeric()
		return phpv.ZFalse.ZVal(), nil
	}

	// Only check strings for numeric-ness; arrays and objects return false
	if z.GetType() != phpv.ZtString {
		return phpv.ZFalse.ZVal(), nil
	}

	s := z.AsString(ctx)
	return phpv.ZBool(s.IsNumeric()).ZVal(), nil
}

// > func bool is_object ( mixed $var )
func fncIsObject(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtObject).ZVal(), nil
}

// > func bool is_real ( mixed $var )
func fncIsReal(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncIsFloat(ctx, args)
}

// > func bool is_resource ( mixed $var )
func fncIsResource(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	if z.GetType() != phpv.ZtResource {
		return phpv.ZFalse.ZVal(), nil
	}
	// Closed resources (ResourceType == ResourceUnknown) are not "open" resources.
	// PHP's is_resource() returns false for closed resources.
	if res, ok := z.Value().(phpv.Resource); ok {
		if res.GetResourceType() == phpv.ResourceUnknown {
			return phpv.ZFalse.ZVal(), nil
		}
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func bool is_scalar ( mixed $var )
func fncIsScalar(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	switch z.GetType() {
	case phpv.ZtInt, phpv.ZtFloat, phpv.ZtString, phpv.ZtBool:
		return phpv.ZBool(true).ZVal(), nil
	}
	return phpv.ZBool(false).ZVal(), nil
}

// > func bool is_string ( mixed $var )
func fncIsString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(z.GetType() == phpv.ZtString).ZVal(), nil
}

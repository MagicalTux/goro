package core

import "errors"

//> func int strlen ( string $string )
func fncStrlen(ctx Context, args []*ZVal) (*ZVal, error) {
	var s ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return ZInt(len(s)).ZVal(), nil
}

//> func int error_reporting ([ int $level ] )
func fncErrorReporting(ctx Context, args []*ZVal) (*ZVal, error) {
	var level *ZInt
	_, err := Expand(ctx, args, &level)
	if err != nil {
		return nil, err
	}

	if level != nil {
		ctx.Global().SetLocalConfig("error_reporting", (*level).ZVal())
	}

	return ctx.GetConfig("error_reporting", ZInt(0).ZVal()), nil
}

//> func bool define ( string $name , mixed $value )
func fncDefine(ctx Context, args []*ZVal) (*ZVal, error) {
	var name ZString
	var value *ZVal
	_, err := Expand(ctx, args, &name, &value)
	if err != nil {
		return nil, err
	}

	g := ctx.Global()

	if _, ok := g.constant[name]; ok {
		// TODO trigger notice: Constant %s already defined
		return ZBool(false).ZVal(), nil
	}

	g.constant[name] = value
	return ZBool(true).ZVal(), nil
}

//> func bool defined ( string $name )
func fncDefined(ctx Context, args []*ZVal) (*ZVal, error) {
	var name ZString
	_, err := Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	g := ctx.Global()

	_, ok := g.constant[name]

	return ZBool(ok).ZVal(), nil
}

//> func int count ( mixed $array_or_countable [, int $mode = COUNT_NORMAL ] )
func fncCount(ctx Context, args []*ZVal) (*ZVal, error) {
	var countable *ZVal
	var mode *ZInt
	_, err := Expand(ctx, args, &countable, &mode)
	if err != nil {
		return nil, err
	}

	if mode != nil {
		return nil, errors.New("todo recursive count")
	}

	if v, ok := countable.Value().(ZCountable); ok {
		return v.Count(ctx).ZVal(), nil
	}

	// make this a warning
	return ZInt(1).ZVal(), errors.New("count(): Parameter must be an array or an object that implements Countable")
}

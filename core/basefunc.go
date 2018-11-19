package core

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
		ctx.GetGlobal().SetLocalConfig("error_reporting", (*level).ZVal())
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

	g := ctx.GetGlobal()

	if _, ok := g.constant[name]; ok {
		// TODO trigger notice: Constant %s already defined
		return ZBool(false).ZVal(), nil
	}

	g.constant[name] = value
	return ZBool(true).ZVal(), nil
}

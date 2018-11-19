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

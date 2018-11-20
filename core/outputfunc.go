package core

//> func void echo ( string $arg1 [, string $... ] )
func stdFuncEcho(ctx Context, args []*ZVal) (*ZVal, error) {
	for _, z := range args {
		ctx.Write([]byte(z.String()))
	}
	return nil, nil
}

//> func int print ( string $arg )
func fncPrint(ctx Context, args []*ZVal) (*ZVal, error) {
	var s ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	ctx.Write([]byte(s))
	return ZInt(1).ZVal(), nil
}

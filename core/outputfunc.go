package core

import "github.com/MagicalTux/goro/core/phpv"

// > func void echo ( string $arg1 [, string $... ] )
func stdFuncEcho(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	for _, z := range args {
		_, err := ctx.Write([]byte(z.AsString(ctx)))
		if err != nil {
			return nil, ctx.FuncError(err)
		}
	}
	return nil, nil
}

// > func int print ( string $arg )
func fncPrint(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	_, err = ctx.Write([]byte(s))
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	return phpv.ZInt(1).ZVal(), nil
}

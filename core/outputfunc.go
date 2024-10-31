package core

import "github.com/MagicalTux/goro/core/phpv"

// > func void echo ( string $arg1 [, string $... ] )
func stdFuncEcho(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	for _, z := range args {
		ctx.Write([]byte(z.String()))
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

	ctx.Write([]byte(s))
	return phpv.ZInt(1).ZVal(), nil
}

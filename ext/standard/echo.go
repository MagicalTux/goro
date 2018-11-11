package standard

import "github.com/MagicalTux/gophp/core"

//> func void echo ( string $arg1 [, string $... ] )
func stdFuncEcho(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	for _, z := range args {
		ctx.Write([]byte(z.String()))
	}
	return nil, nil
}

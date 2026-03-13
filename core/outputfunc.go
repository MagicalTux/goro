package core

import (
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func void echo ( string $arg1 [, string $... ] )
func stdFuncEcho(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	for _, z := range args {
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		_, err = ctx.Write([]byte(s.Value().(phpv.ZString)))
		if err != nil {
			// Don't wrap PhpThrow errors (e.g. from output buffer callbacks)
			// — they need to remain catchable by try/catch
			if _, ok := err.(*phperr.PhpThrow); ok {
				return nil, err
			}
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
		if _, ok := err.(*phperr.PhpThrow); ok {
			return nil, err
		}
		return nil, ctx.FuncError(err)
	}
	return phpv.ZInt(1).ZVal(), nil
}

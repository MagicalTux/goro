package openssl

import (
	"crypto/rand"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string openssl_random_pseudo_bytes ( int $length [, bool &$crypto_strong ] )
func fncOpensslRandomPseudoBytes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var length phpv.ZInt

	_, err := core.Expand(ctx, args, &length)
	if err != nil {
		return nil, err
	}

	if length < 1 {
		ctx.Warn("openssl_random_pseudo_bytes(): Length must be greater than 0")
		return phpv.ZBool(false).ZVal(), nil
	}

	buf := make([]byte, int(length))
	_, err = rand.Read(buf)
	if err != nil {
		// Set &$crypto_strong to false if provided
		if len(args) > 1 && args[1] != nil {
			name := args[1].GetName()
			falseVal := phpv.ZBool(false).ZVal()
			falseVal.Name = &name
			ctx.Parent(1).OffsetSet(ctx, name, falseVal)
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// Set &$crypto_strong to true if provided
	if len(args) > 1 && args[1] != nil {
		name := args[1].GetName()
		trueVal := phpv.ZBool(true).ZVal()
		trueVal.Name = &name
		ctx.Parent(1).OffsetSet(ctx, name, trueVal)
	}

	return phpv.ZString(buf).ZVal(), nil
}

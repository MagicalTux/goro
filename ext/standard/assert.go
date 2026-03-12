package standard

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool assert ( mixed $assertion )
func fncAssert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("assert() expects at least 1 argument, 0 given")
	}

	assertion := args[0]

	if !assertion.AsBool(ctx) {
		// Throw AssertionError
		return nil, phpobj.ThrowError(ctx, phpobj.AssertionError, "assert(false)")
	}

	return phpv.ZBool(true).ZVal(), nil
}

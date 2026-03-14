package standard

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool assert ( mixed $assertion [, Throwable|string|null $description = null] )
func fncAssert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("assert() expects at least 1 argument, 0 given")
	}

	// Check zend.assertions INI setting
	// -1 = completely disabled (no evaluation), 0 = disabled (no action), 1 = enabled
	zendAssertions := ctx.GetConfig("zend.assertions", phpv.ZInt(1).ZVal()).AsInt(ctx)
	if zendAssertions <= 0 {
		return phpv.ZBool(true).ZVal(), nil
	}

	assertion := args[0]

	if !assertion.AsBool(ctx) {
		// Check assert.exception INI setting (default is 1 in PHP 8)
		assertException := ctx.GetConfig("assert.exception", phpv.ZInt(1).ZVal()).AsInt(ctx)

		// Handle the description argument
		var description *phpv.ZVal
		if len(args) >= 2 {
			description = args[1]
		}

		if assertException != 0 {
			// When assert.exception=1, throw an exception
			if description != nil && description.GetType() == phpv.ZtObject {
				// If description is a Throwable object, throw it directly
				return nil, phpobj.ThrowObject(ctx, description)
			}

			// Use description string as the message, or default to "assert(false)"
			msg := "assert(false)"
			if description != nil && !description.IsNull() {
				msg = description.AsString(ctx).String()
			}
			return nil, phpobj.ThrowError(ctx, phpobj.AssertionError, msg)
		}

		// When assert.exception=0, issue a warning
		msg := "assert(false)"
		if description != nil && !description.IsNull() && description.GetType() == phpv.ZtString {
			msg = description.AsString(ctx).String()
		}
		err := ctx.Warn("assert(): %s failed", msg)
		if err != nil {
			return nil, err
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

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

		// When assert.exception=0, issue a warning and optionally call callback
		msg := "assert(false)"
		if description != nil && !description.IsNull() && description.GetType() == phpv.ZtString {
			msg = description.AsString(ctx).String()
		}

		// Check assert.warning (default 1)
		assertWarning := ctx.GetConfig("assert.warning", phpv.ZInt(1).ZVal()).AsInt(ctx)
		if assertWarning != 0 {
			ctx.Warn("%s failed", msg)
		}

		// Check assert.callback - stored as a callable via assert_options()
		callbackVal := ctx.GetConfig("assert.callback", phpv.ZNULL.ZVal())
		if callbackVal != nil && !callbackVal.IsNull() {
			loc := ctx.Loc()
			file := ""
			line := phpv.ZInt(0)
			if loc != nil {
				file = loc.Filename
				line = phpv.ZInt(loc.Line)
			}
			callbackArgs := []*phpv.ZVal{
				phpv.ZString(file).ZVal(),
				line.ZVal(),
				phpv.ZString(msg).ZVal(),
			}

			// Resolve the callable. For string callbacks, look up by function name.
			// For closure/object callbacks, extract the Callable interface.
			if callbackVal.GetType() == phpv.ZtString {
				funcName := callbackVal.AsString(ctx)
				callable, resolveErr := ctx.Global().GetFunction(ctx, funcName)
				if resolveErr != nil {
					return nil, resolveErr
				}
				_, callErr := ctx.CallZVal(ctx, callable, callbackArgs)
				if callErr != nil {
					return nil, callErr
				}
			} else if callbackVal.GetType() == phpv.ZtObject {
				// For closures and invokable objects, invoke via __invoke method
				if obj, ok := callbackVal.Value().(phpv.ZObject); ok {
					if f, hasInvoke := obj.GetClass().GetMethod("__invoke"); hasInvoke {
						_, callErr := ctx.CallZVal(ctx, f.Method, callbackArgs, obj)
						if callErr != nil {
							return nil, callErr
						}
					}
				}
			} else if callable, ok := callbackVal.Value().(phpv.Callable); ok {
				_, callErr := ctx.CallZVal(ctx, callable, callbackArgs)
				if callErr != nil {
					return nil, callErr
				}
			}
		}

		// Check assert.bail (default 0)
		assertBail := ctx.GetConfig("assert.bail", phpv.ZInt(0).ZVal()).AsInt(ctx)
		if assertBail != 0 {
			return nil, phpv.ExitError(0)
		}

		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

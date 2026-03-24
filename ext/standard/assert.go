package standard

import (
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	ASSERT_ACTIVE     phpv.ZInt = 1
	ASSERT_WARNING    phpv.ZInt = 2
	ASSERT_CALLBACK   phpv.ZInt = 3
	ASSERT_BAIL       phpv.ZInt = 4
	ASSERT_QUIET_EVAL phpv.ZInt = 5
	ASSERT_EXCEPTION  phpv.ZInt = 6
)

// assert_options INI key mapping
var assertOptionKeys = map[phpv.ZInt]string{
	ASSERT_ACTIVE:    "assert.active",
	ASSERT_WARNING:   "assert.warning",
	ASSERT_CALLBACK:  "assert.callback",
	ASSERT_BAIL:      "assert.bail",
	ASSERT_QUIET_EVAL: "assert.quiet_eval",
	ASSERT_EXCEPTION: "assert.exception",
}

// > func mixed assert_options ( int $option [, mixed $value ] )
func fncAssertOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("assert_options() expects at least 1 argument, 0 given")
	}

	// Emit deprecation warning (PHP 8.3+)
	_ = ctx.Deprecated("Function assert_options() is deprecated since 8.3", logopt.NoFuncName(true))

	option := args[0].AsInt(ctx)
	iniKey, ok := assertOptionKeys[option]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "assert_options(): Argument #1 ($option) must be an ASSERT_* constant")
	}

	// Get the current value
	oldVal := ctx.GetConfig(phpv.ZString(iniKey), phpv.ZInt(0).ZVal())

	// If a new value is provided, set it
	if len(args) >= 2 {
		ctx.Global().SetLocalConfig(phpv.ZString(iniKey), args[1])
	}

	return oldVal, nil
}

// > func bool assert ( mixed $assertion [, Throwable|string|null $description = null] )
func fncAssert(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.Errorf("assert() expects at least 1 argument, 0 given")
	}

	// Check assert.active INI setting (default is 1)
	// When assert.active = 0, assert() always returns true without evaluating
	assertActive := ctx.GetConfig("assert.active", phpv.ZInt(1).ZVal()).AsInt(ctx)
	if assertActive == 0 {
		return phpv.ZBool(true).ZVal(), nil
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

			// Use description string as the message
			msg := "assert(false)"
			if description != nil && description.IsNull() {
				msg = "Assertion failed"
			} else if description != nil && !description.IsNull() {
				msg = description.AsString(ctx).String()
			}
			return nil, phpobj.ThrowError(ctx, phpobj.AssertionError, msg)
		}

		// When assert.exception=0, issue a warning and optionally call callback
		// When description is explicitly null or not provided:
		//   - PHP 8.5+ with null: "Assertion failed"
		//   - PHP 8.5+ without description: "assert(expression) failed" (we don't have expression, use generic)
		msg := "Assertion failed"
		if description != nil && !description.IsNull() && description.GetType() == phpv.ZtString {
			msg = description.AsString(ctx).String()
		} else if description == nil {
			// No description argument provided - use assert(false) format
			// (In PHP this would show the actual expression, but we don't have it)
			msg = "assert(false)"
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
			var callbackErr error
			if callbackVal.GetType() == phpv.ZtString {
				funcName := callbackVal.AsString(ctx)
				callable, resolveErr := ctx.Global().GetFunction(ctx, funcName)
				if resolveErr != nil {
					callbackErr = resolveErr
				} else {
					_, callErr := ctx.CallZVal(ctx, callable, callbackArgs)
					if callErr != nil {
						callbackErr = callErr
					}
				}
			} else if callbackVal.GetType() == phpv.ZtObject {
				// For closures and invokable objects, invoke via __invoke method
				if obj, ok := callbackVal.Value().(phpv.ZObject); ok {
					if f, hasInvoke := obj.GetClass().GetMethod("__invoke"); hasInvoke {
						_, callErr := ctx.CallZVal(ctx, f.Method, callbackArgs, obj)
						if callErr != nil {
							callbackErr = callErr
						}
					}
				}
			} else if callable, ok := callbackVal.Value().(phpv.Callable); ok {
				_, callErr := ctx.CallZVal(ctx, callable, callbackArgs)
				if callErr != nil {
					callbackErr = callErr
				}
			}

			// If the callback raised an error, display it as a warning
			if callbackErr != nil {
				if phpErr, ok := callbackErr.(*phpv.PhpError); ok {
					ctx.Warn("Uncaught %s", phpErr.Err.Error())
				} else {
					ctx.Warn("Uncaught error in assert callback: %s", callbackErr.Error())
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

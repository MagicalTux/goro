package core

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool trigger_error ( string $error_msg [, int $error_type = E_USER_NOTICE ] )
func fncTriggerError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var message phpv.ZString
	var errorTypeArg Optional[phpv.ZInt]
	_, err := Expand(ctx, args, &message, &errorTypeArg)
	if err != nil {
		return nil, err
	}

	errorType := errorTypeArg.GetOrDefault(E_USER_NOTICE)

	// Validate error type - must be one of the E_USER_* constants
	switch phpv.PhpErrorType(errorType) {
	case phpv.E_USER_ERROR, phpv.E_USER_WARNING, phpv.E_USER_NOTICE, phpv.E_USER_DEPRECATED:
		// valid
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("trigger_error(): Argument #2 ($error_level) must be one of E_USER_ERROR, E_USER_WARNING, E_USER_NOTICE, or E_USER_DEPRECATED"))
	}

	phpErr := &phpv.PhpError{
		Err:  errors.New(message.String()),
		Code: phpv.PhpErrorType(errorType),
		Loc:  ctx.Loc(),
	}
	err = phperr.HandleUserError(ctx, phpErr)
	if err == phperr.ErrHandledByUser {
		return phpv.ZBool(true).ZVal(), nil
	}
	if err != nil {
		return nil, err
	}

	ctx.LogError(phpErr)
	return phpv.ZBool(true).ZVal(), nil
}

// > func mixed set_error_handler ( callable $error_handler [, int $error_types = E_ALL | E_STRICT ] )
func fncSetErrorHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("set_error_handler() expects at least 1 argument, 0 given")
	}

	// Get previous error handler before setting the new one
	prevErrHandler, _ := ctx.Global().GetUserErrorHandler()

	// PHP accepts null to reset the error handler
	if args[0].IsNull() {
		var errorTypeArg Optional[phpv.ZInt]
		if len(args) > 1 {
			Expand(ctx, args[1:], &errorTypeArg)
		}
		errorType := errorTypeArg.GetOrDefault(E_ALL | E_STRICT)
		ctx.Global().SetUserErrorHandler(nil, phpv.PhpErrorType(errorType))
		if prevErrHandler == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		return prevErrHandler.ZVal(), nil
	}

	var handler phpv.Callable
	var errorTypeArg Optional[phpv.ZInt]
	_, err := Expand(ctx, args, &handler, &errorTypeArg)
	if err != nil {
		return nil, err
	}

	errorType := errorTypeArg.GetOrDefault(E_ALL | E_STRICT)
	ctx.Global().SetUserErrorHandler(handler, phpv.PhpErrorType(errorType))

	if prevErrHandler == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return prevErrHandler.ZVal(), nil
}

// > func callable|null set_exception_handler ( callable|null $exception_handler )
func fncSetExceptionHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "set_exception_handler() expects exactly 1 argument, 0 given")
	}

	// PHP accepts null to reset to default handler
	if args[0].IsNull() {
		prev := ctx.Global().SetUserExceptionHandler(nil, nil)
		if prev == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		return prev, nil
	}

	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		// PHP 8.0+ throws TypeError with specific message format
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("set_exception_handler(): Argument #1 ($callback) must be a valid callback or null, %s",
				callbackErrorMessage(ctx, args[0])))
	}

	// Store the original ZVal so we can return it later (PHP returns
	// the handler in its original form: string for named functions,
	// Closure object for closures, array for [class, method])
	prev := ctx.Global().SetUserExceptionHandler(handler, args[0])
	if prev == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return prev, nil
}

// callbackErrorMessage generates a descriptive error for invalid callbacks.
func callbackErrorMessage(ctx phpv.Context, arg *phpv.ZVal) string {
	switch arg.GetType() {
	case phpv.ZtString:
		return fmt.Sprintf("function \"%s\" not found or invalid function name", arg.String())
	case phpv.ZtArray:
		arr := arg.AsArray(ctx)
		if arr != nil && arr.Count(ctx) == 2 {
			classVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
			if classVal != nil {
				className := classVal.String()
				if className == "" {
					return "class \"\" not found"
				}
			}
		}
		return "no array or string given"
	default:
		return fmt.Sprintf("no array or string given")
	}
}

// > func bool restore_exception_handler ( void )
func fncRestoreExceptionHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Global().RestoreUserExceptionHandler()
	return phpv.ZBool(true).ZVal(), nil
}

// > func bool error_log ( string $message [, int $message_type = 0 [, string $destination [, string $extra_headers ]]] )
func fncErrorLog(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var message phpv.ZString
	var messageType *phpv.ZInt
	var destination *phpv.ZString
	_, err := Expand(ctx, args, &message, &messageType, &destination)
	if err != nil {
		return nil, err
	}

	msgType := Deref(messageType, phpv.ZInt(0))

	switch msgType {
	case 0:
		// Send to PHP's system logger (use Go's log package)
		log.Print(string(message))
		return phpv.ZTrue.ZVal(), nil
	case 3:
		// Append to file
		if destination == nil {
			return phpv.ZFalse.ZVal(), nil
		}
		dest := string(*destination)

		// Check open_basedir
		if err := ctx.Global().CheckOpenBasedir(ctx, dest, "error_log"); err != nil {
			ctx.Warn("error_log(%s): Failed to open stream: Operation not permitted", dest, logopt.NoFuncName(true))
			return phpv.ZFalse.ZVal(), nil
		}

		// Resolve path relative to PHP's virtual cwd
		p := dest
		if len(p) == 0 || p[0] != '/' {
			p = string(ctx.Global().Getwd()) + "/" + p
		}

		f, ferr := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if ferr != nil {
			ctx.Warn("error_log(%s): Failed to open stream: %s", dest, ferr.Error(), logopt.NoFuncName(true))
			return phpv.ZFalse.ZVal(), nil
		}
		defer f.Close()
		_, ferr = f.WriteString(string(message))
		if ferr != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZTrue.ZVal(), nil
	default:
		// Type 1 (email), 2 (remote debugger), 4 (SAPI handler) not implemented
		return phpv.ZTrue.ZVal(), nil
	}
}

package core

import (
	"errors"

	"github.com/MagicalTux/goro/core/phperr"
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
	phpErr := &phpv.PhpError{
		Err:  errors.New(message.String()),
		Code: phpv.PhpErrorType(errorType),
		Loc:  ctx.Loc(),
	}
	err = phperr.HandleUserError(ctx, phpErr)
	if err == phperr.ErrHandledByUser {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ctx.LogError(phpErr)
	return nil, nil
}

// > func mixed set_error_handler ( callable $error_handler [, int $error_types = E_ALL | E_STRICT ] )
func fncSetErrorHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	var errorTypeArg Optional[phpv.ZInt]
	_, err := Expand(ctx, args, &handler, &errorTypeArg)
	if err != nil {
		return nil, err
	}

	errorType := errorTypeArg.GetOrDefault(E_ALL | E_STRICT)
	ctx.Global().SetUserErrorHandler(handler, phpv.PhpErrorType(errorType))

	// TODO: If the previous error handler was a class method,
	// this function will return an indexed array with the class and the method name.

	prevErrHandler, _ := ctx.Global().GetUserErrorHandler()
	return prevErrHandler.ZVal(), err
}

// > func callable|null set_exception_handler ( callable|null $exception_handler )
func fncSetExceptionHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	prev := ctx.Global().SetUserExceptionHandler(handler)
	if prev == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	return prev.ZVal(), nil
}

// > func bool restore_exception_handler ( void )
func fncRestoreExceptionHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Global().SetUserExceptionHandler(nil)
	return phpv.ZBool(true).ZVal(), nil
}

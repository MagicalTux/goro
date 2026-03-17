package phperr

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

// ErrHandledByUser is returned when a user error handler has handled the error.
// Callers should not display the default error message when this is returned.
var ErrHandledByUser = errors.New("handled by user error handler")

func HandleUserError(ctx phpv.Context, err *phpv.PhpError) error {
	var returnErr error = err
	errHandler, filterType := ctx.Global().GetUserErrorHandler()

	// If there's no user error handler, use default behavior
	if errHandler == nil {
		if err.IsNonFatal() {
			return nil
		}
		return returnErr
	}

	if err.Code&filterType == 0 {
		if err.IsNonFatal() {
			return nil
		}
		return returnErr
	}

	if errHandler != nil && err.CanBeUserHandled() {
		// Temporarily pop the user error handler while it's being called
		// to prevent re-entrancy (matching PHP behavior)
		ctx.Global().RestoreUserErrorHandler()

		// PHP includes the function name prefix in $errstr for user error handlers
		errMsg := err.Err.Error()
		if err.FuncName != "" {
			errMsg = err.FuncName + "(): " + errMsg
		}

		args := []*phpv.ZVal{
			phpv.ZInt(err.Code).ZVal(),
			phpv.ZStr(errMsg),
			phpv.ZStr(err.Loc.Filename),
			phpv.ZInt(err.Loc.Line).ZVal(),
		}

		var proceed *phpv.ZVal
		var err2 error
		if err.IsInternal {
			// When the error originates from internal code (e.g., OB callbacks),
			// the error handler frame should show as [internal function] in stack traces.
			proceed, err2 = ctx.CallZValInternal(ctx, errHandler, args)
		} else {
			proceed, err2 = ctx.CallZVal(ctx, errHandler, args)
		}

		// Restore the user error handler by pushing it back
		ctx.Global().SetUserErrorHandler(errHandler, filterType)

		if err2 != nil {
			if e, ok := err2.(*PhpThrow); ok {
				class := e.Obj.GetClass()
				if stack, ok := e.Obj.GetOpaque(class).([]*phpv.StackTraceEntry); ok {
					// remove the user handler frame from the stack
					stack = stack[1:]
					e.Obj.SetOpaque(class, stack)
				}
			}
			returnErr = err2
		} else if proceed != nil && proceed.GetType() == phpv.ZtBool && !bool(proceed.Value().(phpv.ZBool)) {
			// Handler explicitly returned false: continue with default error handler
			// (don't suppress the standard error message)
			if err.IsNonFatal() {
				return nil
			}
		} else if err.IsNonFatal() {
			// Handler returned void/null/true: suppress default output
			return ErrHandledByUser
		}
	}

	return returnErr
}

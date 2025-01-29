package phperr

import (
	"github.com/MagicalTux/goro/core/phpv"
)

func HandleUserError(ctx phpv.Context, err *phpv.PhpError) error {
	var returnErr error = err
	errHandler, filterType := ctx.Global().GetUserErrorHandler()

	if err.Code&filterType == 0 {
		if err.IsNonFatal() {
			return nil
		}
		return returnErr
	}

	if errHandler != nil && err.CanBeUserHandled() {
		args := []*phpv.ZVal{
			phpv.ZInt(err.Code).ZVal(),
			phpv.ZStr(err.Err.Error()),
			phpv.ZStr(err.Loc.Filename),
			phpv.ZInt(err.Loc.Line).ZVal(),
		}

		proceed, err2 := ctx.CallZVal(ctx, errHandler, args)

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
		} else if bool(proceed.AsBool(ctx)) || err.IsNonFatal() {
			returnErr = nil
		}
	}

	return returnErr
}

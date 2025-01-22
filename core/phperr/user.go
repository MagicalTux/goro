package phperr

import "github.com/MagicalTux/goro/core/phpv"

func HandleUserError(ctx phpv.Context, err *phpv.PhpError) error {
	var returnErr error = err
	errHandler, filterType := ctx.Global().GetUserErrorHandler()

	if err.Code&filterType == 0 {
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
			returnErr = err2
		} else if proceed.AsBool(ctx) {
			returnErr = nil
		}
	}
	return returnErr
}

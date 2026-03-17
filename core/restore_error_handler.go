package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool restore_error_handler ( void )
func fncRestoreErrorHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Global().RestoreUserErrorHandler()
	return phpv.ZBool(true).ZVal(), nil
}

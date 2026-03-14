package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool restore_error_handler ( void )
func fncRestoreErrorHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Reset the user error handler to nil (no user handler).
	// In PHP this pops a stack of error handlers, but most real-world
	// usage just needs a simple reset. The filter is set to 0 so no
	// user error types are intercepted.
	ctx.Global().SetUserErrorHandler(nil, 0)
	return phpv.ZBool(true).ZVal(), nil
}

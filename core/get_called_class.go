package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string|false get_called_class ( void )
func fncGetCalledClass(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Walk up to the caller's context to get late static binding info.
	// get_called_class itself runs in its own FuncContext, so we need
	// the parent context (the function/method that called get_called_class).
	parent := ctx.Parent(1)
	if parent != nil {
		if cc, ok := parent.(interface{ CalledClass() phpv.ZClass }); ok {
			if called := cc.CalledClass(); called != nil {
				return called.GetName().ZVal(), nil
			}
		}
		// Fall back to the parent's class
		pClass := parent.Class()
		if pClass != nil {
			return pClass.GetName().ZVal(), nil
		}
	}
	class := ctx.Class()
	if class == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return class.GetName().ZVal(), nil
}

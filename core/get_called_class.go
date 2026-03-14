package core

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string|false get_called_class ( void )
func fncGetCalledClass(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	class := ctx.Class()
	if class == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return class.GetName().ZVal(), nil
}

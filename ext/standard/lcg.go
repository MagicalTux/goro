package standard

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func number lcg_value ()
func fncLcgValue(ctx phpv.Context, _ []*phpv.ZVal) (*phpv.ZVal, error) {
	r := ctx.Global().Random()
	return phpv.ZFloat(r.Lcg.Next()).ZVal(), nil
}

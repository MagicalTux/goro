package standard

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func number lcg_value ()
func fncLcgValue(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) > 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("lcg_value() expects exactly 0 arguments, %d given", len(args)))
	}
	r := ctx.Global().Random()
	return phpv.ZFloat(r.Lcg.Next()).ZVal(), nil
}

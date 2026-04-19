package standard

import (
	"time"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func bool set_time_limit ( int $seconds )
func fncSetTimeLimit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var d phpv.ZInt
	_, err := core.Expand(ctx, args, &d)
	if err != nil {
		return nil, err
	}

	// In PHP, set_time_limit(0) means "no time limit"
	if d == 0 {
		ctx.Global().(*phpctx.Global).SetDeadline(time.Now().Add(24 * 365 * time.Hour))
	} else {
		ctx.Global().(*phpctx.Global).SetDeadline(time.Now().Add(time.Duration(d) * time.Second))
	}
	return phpv.ZNULL.ZVal(), nil
}

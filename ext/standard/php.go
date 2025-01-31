package standard

import (
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool set_time_limit ( int $seconds )
func fncSetTimeLimit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var d phpv.ZInt
	_, err := core.Expand(ctx, args, &d)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	end := now.Add(time.Duration(d) * time.Second)

	ctx.Global().(*phpctx.Global).SetDeadline(end)
	return phpv.ZNULL.ZVal(), nil
}

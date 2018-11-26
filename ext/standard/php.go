package standard

import (
	"time"

	"github.com/MagicalTux/goro/core"
)

//> func bool set_time_limit ( int $seconds )
func fncSetTimeLimit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var d core.ZInt
	_, err := core.Expand(ctx, args, &d)
	if err != nil {
		return nil, err
	}

	ctx.Global().SetDeadline(time.Now().Add(time.Duration(d) * time.Second))
	return core.ZNULL.ZVal(), nil
}

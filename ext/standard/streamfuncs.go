package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// > func resource stream_context_create ([ array $options [, array $params ]] )
func fncStreamContextCreate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var options core.Optional[*phpv.ZArray]
	var params core.Optional[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &options, &params)
	if err != nil {
		return nil, err
	}

	streamCtx := stream.FromZArray(ctx, options.Get(), params.Get())
	return streamCtx.ZVal(), nil
}

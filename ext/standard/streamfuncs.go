package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
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

	streamCtx := stream.NewContextFromZArray(ctx, options.Get(), params.Get())
	return streamCtx.ZVal(), nil
}

// > func resource stream_context_get_default ([ array $options ] )
// > alias stream_context_set_default
// behaves like stream_context_set_default() if $options is provided,
// which persistently modifies the default stream context
func fncStreamContextGetDefault(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var options core.Optional[*phpv.ZArray]
	_, err := core.Expand(ctx, args, &options)
	if err != nil {
		return nil, err
	}

	g := ctx.Global().(*phpctx.Global)
	if g.DefaultStreamContext == nil {
		g.DefaultStreamContext = &stream.Context{
			ID:      ctx.Global().NextResourceID(),
			Options: make(map[phpv.ZString]stream.ContextOptions),
		}
	}

	streamCtx := g.DefaultStreamContext
	if options.HasArg() {
		for wrapperName, wrapperOptions := range options.Get().Iterate(ctx) {
			for key, val := range wrapperOptions.AsArray(ctx).Iterate(ctx) {
				streamCtx.SetOption(wrapperName.AsString(ctx), key.AsString(ctx), val)
			}
		}
	}

	return g.DefaultStreamContext.ZVal(), nil
}

// > func array stream_context_get_options ( resource $stream_or_context )
func fncStreamContextGetOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var streamOrContext phpv.Resource
	_, err := core.Expand(ctx, args, &streamOrContext)
	if err != nil {
		return nil, err
	}

	var streamCtx *stream.Context
	switch t := streamOrContext.(type) {
	case *stream.Stream:
		streamCtx = t.Context
	case *stream.Context:
		streamCtx = t
	default:
		return nil, ctx.FuncErrorf("invalid argument, must be stream or context")
	}

	if streamCtx == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	options, _ := streamCtx.ToZArray(ctx)
	return options.ZVal(), nil
}

// > func bool stream_context_set_option ( resource $stream_or_context , string $wrapper , string $option , mixed $value )
func fncStreamContextSetOption(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var streamOrContext phpv.Resource
	var wrapper phpv.ZString
	var option phpv.ZString
	var value *phpv.ZVal
	_, err := core.Expand(ctx, args, &streamOrContext, &wrapper, &option, &value)
	if err != nil {
		return nil, err
	}

	switch t := streamOrContext.(type) {
	default:
		return nil, ctx.FuncErrorf("invalid argument, must be stream or context")

	case *stream.Stream:
		if t.Context == nil {
			t.Context = stream.NewContext(ctx)
		}
		t.Context.SetOption(wrapper, option, value)
	case *stream.Context:
		t.SetOption(wrapper, option, value)
	}

	return nil, nil
}

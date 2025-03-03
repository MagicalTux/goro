package stream

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

type ContextOptions map[phpv.ZString]*phpv.ZVal

type Context struct {
	ID int

	Options map[ /*wrapperName*/ phpv.ZString]ContextOptions
	// ... where wrapperName can be: http, ftp, curl and others,
	// see stream_context_create for list of known wrapper names.
	// Map is used to allowed extension of new wrappers.

	NotifParam phpv.Callable
}

func (c *Context) String() string { return "stream-context" }

func (c *Context) GetType() phpv.ZType { return phpv.ZtResource }

func (c *Context) ZVal() *phpv.ZVal { return phpv.NewZVal(c) }

func (c *Context) Value() phpv.Val { return c }

func (c *Context) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtString:
		return phpv.ZStr(fmt.Sprintf("Resource id #%d", c.ID)), nil
	case phpv.ZtResource:
		return c.ZVal(), nil
	default:
		return phpv.ZInt(c.ID).AsVal(ctx, t)
	}
}

func (c *Context) GetResourceType() phpv.ResourceType {
	return phpv.ResourceContext

}
func (c *Context) GetResourceID() int {
	return c.ID
}

func FromZArray(ctx phpv.Context, options *phpv.ZArray, params *phpv.ZArray) *Context {
	streamCtx := &Context{
		ID:      ctx.Global().NextResourceID(),
		Options: make(map[phpv.ZString]ContextOptions),
	}
	if options != nil {
		for k1, entries := range options.Iterate(ctx) {
			wrapperName := k1.AsString(ctx)
			if _, ok := streamCtx.Options[wrapperName]; !ok {
				streamCtx.Options[wrapperName] = ContextOptions{}
			}

			for option, value := range entries.AsArray(ctx).Iterate(ctx) {
				option := option.AsString(ctx)
				streamCtx.Options[wrapperName][option] = value
			}
		}
	}

	if params != nil {
		v, _ := params.OffsetGet(ctx, phpv.ZStr("notification"))
		if fn, ok := v.Value().(phpv.Callable); ok {
			streamCtx.NotifParam = fn
		}
	}

	return streamCtx
}

func (c *Context) ToZArray(ctx phpv.Context) (options *phpv.ZArray, params *phpv.ZArray) {
	options = phpv.NewZArray()
	params = phpv.NewZArray()

	for wrapperName, entries := range c.Options {
		wrapperOptions := phpv.NewZArray()
		options.OffsetSet(ctx, wrapperName.ZVal(), wrapperOptions.ZVal())
		for k, v := range entries {
			wrapperOptions.OffsetSet(ctx, k.ZVal(), v)
		}
	}

	if c.NotifParam != nil {
		params.OffsetSet(ctx, phpv.ZStr("notification"), c.NotifParam.ZVal())
	}

	return options, params
}

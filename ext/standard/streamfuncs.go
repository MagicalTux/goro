package standard

import (
	"errors"
	"io"
	"os"
	"strings"

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

// > func bool stream_isatty ( resource $stream )
func fncStreamIsatty(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var streamRes phpv.Resource
	_, err := core.Expand(ctx, args, &streamRes)
	if err != nil {
		return nil, err
	}

	// In the test/embedded context, streams are never TTYs
	return phpv.ZFalse.ZVal(), nil
}

// > func bool stream_wrapper_register ( string $protocol , string $classname [, int $flags = 0 ] )
func fncStreamWrapperRegister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var protocol phpv.ZString
	var className phpv.ZString
	_, err := core.Expand(ctx, args, &protocol, &className)
	if err != nil {
		return nil, err
	}

	g := ctx.Global().(*phpctx.Global)
	handler := stream.NewUserStreamHandler(className)
	g.RegisterStreamHandler(string(protocol), handler)

	return phpv.ZTrue.ZVal(), nil
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

func fncTmpfile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	f, err := os.CreateTemp("", "php")
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	os.Remove(f.Name())
	s := stream.NewStream(f)
	s.SetAttr("wrapper_type", "plainfile")
	s.SetAttr("stream_type", "STDIO")
	s.SetAttr("mode", "r+b")
	s.SetAttr("seekable", true)
	s.SetAttr("uri", f.Name())
	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()
	return s.ZVal(), nil
}

func fncStreamGetMetaData(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	s, ok := handle.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZStr("timed_out"), phpv.ZFalse.ZVal())
	result.OffsetSet(ctx, phpv.ZStr("blocked"), phpv.ZTrue.ZVal())
	result.OffsetSet(ctx, phpv.ZStr("eof"), phpv.ZBool(s.Eof()).ZVal())
	wrapperType := "plainfile"
	if v, ok := s.Attr("wrapper_type").(string); ok {
		wrapperType = v
	}
	result.OffsetSet(ctx, phpv.ZStr("wrapper_type"), phpv.ZString(wrapperType).ZVal())
	streamType := "STDIO"
	if v, ok := s.Attr("stream_type").(string); ok {
		streamType = v
	}
	result.OffsetSet(ctx, phpv.ZStr("stream_type"), phpv.ZString(streamType).ZVal())
	streamMode := "r"
	if v, ok := s.Attr("mode").(string); ok {
		streamMode = v
	}
	result.OffsetSet(ctx, phpv.ZStr("mode"), phpv.ZString(streamMode).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("unread_bytes"), phpv.ZInt(0).ZVal())
	seekable := false
	if v, ok := s.Attr("seekable").(bool); ok {
		seekable = v
	}
	result.OffsetSet(ctx, phpv.ZStr("seekable"), phpv.ZBool(seekable).ZVal())
	if uri, ok := s.Attr("uri").(string); ok {
		result.OffsetSet(ctx, phpv.ZStr("uri"), phpv.ZString(uri).ZVal())
	}
	return result.ZVal(), nil
}

func fncStreamIsLocal(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, errors.New("stream_is_local() expects exactly 1 argument")
	}
	z := args[0]
	if z.GetType() == phpv.ZtString {
		sv := z.String()
		if strings.HasPrefix(sv, "file://") || !strings.Contains(sv, "://") {
			return phpv.ZTrue.ZVal(), nil
		}
		return phpv.ZFalse.ZVal(), nil
	}
	if z.GetType() == phpv.ZtResource {
		if res, ok := z.Value().(phpv.Resource); ok {
			if ss, ok := res.(*stream.Stream); ok {
				if wt, ok := ss.Attr("wrapper_type").(string); ok {
					if wt == "plainfile" || wt == "PHP" {
						return phpv.ZTrue.ZVal(), nil
					}
				}
			}
		}
	}
	return phpv.ZFalse.ZVal(), nil
}

func fncStreamCopyToStream(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var source, dest phpv.Resource
	var maxLength core.Optional[phpv.ZInt]
	var offset core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &source, &dest, &maxLength, &offset)
	if err != nil {
		return nil, err
	}
	srcStream, ok := source.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	dstStream, ok := dest.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	if offset.HasArg() && offset.Get() > 0 {
		srcStream.Seek(int64(offset.Get()), io.SeekStart)
	}
	var n int64
	if maxLength.HasArg() && maxLength.Get() >= 0 {
		n, err = io.CopyN(dstStream, srcStream, int64(maxLength.Get()))
	} else {
		n, err = io.Copy(dstStream, srcStream)
	}
	if err != nil && err != io.EOF {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

func fncStreamGetLine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var length phpv.ZInt
	var ending *phpv.ZString
	_, err := core.Expand(ctx, args, &handle, &length, &ending)
	if err != nil {
		return nil, err
	}
	file, ok := handle.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	endStr := "\n"
	if ending != nil && len(*ending) > 0 {
		endStr = string(*ending)
	}
	var buf []byte
	maxLen := int(length)
	// In PHP, length=0 means "unlimited" - read until delimiter or EOF
	unlimited := maxLen <= 0
	for i := 0; unlimited || i < maxLen; i++ {
		b, berr := file.ReadByte()
		if berr != nil {
			break
		}
		buf = append(buf, b)
		if len(buf) >= len(endStr) {
			tail := string(buf[len(buf)-len(endStr):])
			if tail == endStr {
				buf = buf[:len(buf)-len(endStr)]
				break
			}
		}
	}
	if len(buf) == 0 && file.Eof() {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZString(buf).ZVal(), nil
}

// > func bool stream_context_set_params ( resource $stream_or_context , array $params )
func fncStreamContextSetParams(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var params *phpv.ZArray
	_, err := core.Expand(ctx, args, &handle, &params)
	if err != nil {
		return nil, err
	}
	streamCtx, ok := handle.(*stream.Context)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	if params != nil {
		streamCtx.SetParams(ctx, params)
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func array stream_context_get_params ( resource $stream_or_context )
func fncStreamContextGetParams(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}
	streamCtx, ok := handle.(*stream.Context)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	return streamCtx.GetParams(ctx).ZVal(), nil
}

// > func array stream_get_filters ( void )
func fncStreamGetFilters(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Return a basic list of built-in stream filters
	result := phpv.NewZArray()
	builtinFilters := []string{
		"string.rot13",
		"string.toupper",
		"string.tolower",
		"string.strip_tags",
		"convert.iconv.*",
		"convert.base64-encode",
		"convert.base64-decode",
		"convert.quoted-printable-encode",
		"convert.quoted-printable-decode",
		"zlib.*",
		"bzip2.*",
	}
	for _, f := range builtinFilters {
		result.OffsetSet(ctx, nil, phpv.ZString(f).ZVal())
	}
	return result.ZVal(), nil
}

// > func bool stream_wrapper_unregister ( string $protocol )
func fncStreamWrapperUnregister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Stub: we don't have user stream wrappers yet, but acknowledge the call
	return phpv.ZTrue.ZVal(), nil
}

// > func bool stream_wrapper_restore ( string $protocol )
func fncStreamWrapperRestore(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZTrue.ZVal(), nil
}

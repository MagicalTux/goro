package standard

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// getFilterRegistry returns the per-request stream filter registry from the context
func getFilterRegistry(ctx phpv.Context) *stream.FilterRegistry {
	if g, ok := ctx.Global().(*phpctx.Global); ok && g.StreamFilterRegistry != nil {
		return g.StreamFilterRegistry
	}
	return getFilterRegistry(ctx)
}

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
		"dechunk",
		"zlib.*",
		"bzip2.*",
	}
	for _, f := range builtinFilters {
		result.OffsetSet(ctx, nil, phpv.ZString(f).ZVal())
	}
	// Add registered user filters
	for _, f := range getFilterRegistry(ctx).GetAll() {
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

// streamFilterAttach is the shared implementation for stream_filter_append and stream_filter_prepend
func streamFilterAttach(ctx phpv.Context, args []*phpv.ZVal, prepend bool) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var filtername phpv.ZString
	var readWrite core.Optional[phpv.ZInt]
	var params core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &handle, &filtername, &readWrite, &params)
	if err != nil {
		return nil, err
	}

	funcName := "stream_filter_append"
	if prepend {
		funcName = "stream_filter_prepend"
	}

	s, ok := handle.(*stream.Stream)
	if !ok {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): Argument #1 ($stream) is not a valid stream resource", funcName)
	}

	// Determine direction
	dir := stream.FilterAll
	if readWrite.HasArg() {
		dir = stream.FilterDirection(readWrite.Get())
	}

	// Extract params as a map for built-in filters
	filterParams := extractFilterParams(ctx, params)

	// Try to create a built-in filter first
	name := string(filtername)
	filter := stream.CreateBuiltinFilter(name, filterParams)

	if filter == nil {
		// Try user-registered filter
		className, found := getFilterRegistry(ctx).Lookup(name)
		if !found {
			ctx.Warn("Unable to create or locate filter \"%s\"", name)
			return phpv.ZFalse.ZVal(), nil
		}

		// Get the class
		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		if err != nil {
			ctx.Warn("Unable to create or locate filter \"%s\"", name)
			return phpv.ZFalse.ZVal(), nil
		}

		// Create an instance
		obj, err := phpobj.NewZObject(ctx, class)
		if err != nil {
			ctx.Warn("Unable to create or locate filter \"%s\"", name)
			return phpv.ZFalse.ZVal(), nil
		}

		// Set filtername and params on the object
		obj.OffsetSet(ctx, phpv.ZStr("filtername"), phpv.ZString(name).ZVal())
		if params.HasArg() {
			obj.OffsetSet(ctx, phpv.ZStr("params"), params.Get())
		} else {
			obj.OffsetSet(ctx, phpv.ZStr("params"), phpv.ZStr("").ZVal())
		}

		// Call onCreate()
		onCreateResult, err := obj.CallMethod(ctx, "onCreate")
		if err != nil {
			ctx.Warn("Unable to create or locate filter \"%s\"", name)
			return phpv.ZFalse.ZVal(), nil
		}
		if onCreateResult != nil && !onCreateResult.AsBool(ctx) {
			ctx.Warn("Unable to create or locate filter \"%s\"", name)
			return phpv.ZFalse.ZVal(), nil
		}

		var paramsVal *phpv.ZVal
		if params.HasArg() {
			paramsVal = params.Get()
		}

		// Use the Global context so the filter survives beyond the current function scope
		globalCtx, ok := ctx.Global().(phpv.Context)
		if !ok {
			globalCtx = ctx
		}
		filter = stream.NewUserFilter(globalCtx, obj, s, name, paramsVal)
	}

	// Create the filter resource
	filterRes := &stream.StreamFilterResource{
		ResourceID:   ctx.Global().NextResourceID(),
		ResourceType: phpv.ResourceStreamFilter,
		FilterName:   name,
		Direction:    dir,
		Filter:       filter,
		Stream:       s,
	}

	// Attach the filter - use Global context so it persists
	globalCtx, ok := ctx.Global().(phpv.Context)
	if !ok {
		globalCtx = ctx
	}
	s.SetFilterCtx(globalCtx)
	if dir&stream.FilterRead != 0 {
		s.AddReadFilter(filterRes, prepend)
	}
	if dir&stream.FilterWrite != 0 {
		s.AddWriteFilter(filterRes, prepend)
	}

	return filterRes.ZVal(), nil
}

// extractFilterParams converts the optional params ZVal into a map for built-in filters
func extractFilterParams(ctx phpv.Context, params core.Optional[*phpv.ZVal]) map[string]interface{} {
	result := make(map[string]interface{})
	if !params.HasArg() || params.Get() == nil || params.Get().GetType() == phpv.ZtNull {
		return result
	}
	p := params.Get()
	if p.GetType() == phpv.ZtArray {
		arr := p.AsArray(ctx)
		for k, v := range arr.Iterate(ctx) {
			key := string(k.AsString(ctx))
			switch v.GetType() {
			case phpv.ZtInt:
				result[key] = int(v.AsInt(ctx))
			case phpv.ZtString:
				result[key] = string(v.AsString(ctx))
			case phpv.ZtBool:
				result[key] = bool(v.AsBool(ctx))
			default:
				result[key] = v.String()
			}
		}
	}
	return result
}

// > func resource stream_filter_append ( resource $stream , string $filtername [, int $read_write [, mixed $params ]] )
func fncStreamFilterAppend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return streamFilterAttach(ctx, args, false)
}

// > func resource stream_filter_prepend ( resource $stream , string $filtername [, int $read_write [, mixed $params ]] )
func fncStreamFilterPrepend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return streamFilterAttach(ctx, args, true)
}

// > func bool stream_filter_remove ( resource $stream_filter )
func fncStreamFilterRemove(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	_, err := core.Expand(ctx, args, &handle)
	if err != nil {
		return nil, err
	}

	filterRes, ok := handle.(*stream.StreamFilterResource)
	if !ok || filterRes.Removed {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"stream_filter_remove(): supplied resource is not a valid stream filter resource")
	}

	// Check if the stream is still valid
	if filterRes.Stream == nil || filterRes.Stream.GetResourceType() == phpv.ResourceUnknown {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"stream_filter_remove(): supplied resource is not a valid stream filter resource")
	}

	// Remove the filter from the stream
	filterRes.Stream.RemoveFilter(filterRes)
	filterRes.Removed = true

	// Call onClose for user filters
	if uf, ok := filterRes.Filter.(*stream.UserFilter); ok {
		uf.OnClose()
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func bool stream_filter_register ( string $filter_name , string $class_name )
func fncStreamFilterRegister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filterName phpv.ZString
	var className phpv.ZString
	_, err := core.Expand(ctx, args, &filterName, &className)
	if err != nil {
		return nil, err
	}

	// Validate arguments
	if len(filterName) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"stream_filter_register(): Argument #1 ($filter_name) must be a non-empty string")
	}
	if len(className) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"stream_filter_register(): Argument #2 ($class) must be a non-empty string")
	}

	// Check if it's already a built-in filter name
	if stream.IsBuiltinFilter(string(filterName)) {
		return phpv.ZFalse.ZVal(), nil
	}

	// Register
	ok := getFilterRegistry(ctx).Register(string(filterName), string(className))
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func object stream_bucket_new ( resource $stream , string $buffer )
func fncStreamBucketNew(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var buffer phpv.ZString
	_, err := core.Expand(ctx, args, &handle, &buffer)
	if err != nil {
		return nil, err
	}

	// Create a bucket object with data and datalen properties
	obj, err := phpobj.NewZObject(ctx, phpobj.StdClass)
	if err != nil {
		return nil, err
	}
	obj.OffsetSet(ctx, phpv.ZStr("data"), phpv.ZString(buffer).ZVal())
	obj.OffsetSet(ctx, phpv.ZStr("datalen"), phpv.ZInt(len(buffer)).ZVal())
	return obj.ZVal(), nil
}

// > func void stream_bucket_append ( resource $brigade , object $bucket )
func fncStreamBucketAppend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.FuncErrorf("expects exactly 2 arguments")
	}

	brigade := args[0].Value()
	bb, ok := brigade.(*stream.BucketBrigade)
	if !ok {
		return nil, nil
	}

	bucketZVal := args[1]
	if bucketZVal.GetType() == phpv.ZtObject {
		obj := bucketZVal.Value().(*phpobj.ZObject)
		bb.AppendBucketObj(obj)
	}
	return nil, nil
}

// > func void stream_bucket_prepend ( resource $brigade , object $bucket )
func fncStreamBucketPrepend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, ctx.FuncErrorf("expects exactly 2 arguments")
	}

	brigade := args[0].Value()
	bb, ok := brigade.(*stream.BucketBrigade)
	if !ok {
		return nil, nil
	}

	bucketZVal := args[1]
	if bucketZVal.GetType() == phpv.ZtObject {
		obj := bucketZVal.Value().(*phpobj.ZObject)
		bb.PrependBucketObj(obj)
	}
	return nil, nil
}

// > func object stream_bucket_make_writeable ( resource $brigade )
func fncStreamBucketMakeWriteable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, ctx.FuncErrorf("expects exactly 1 argument")
	}

	brigade := args[0].Value()
	bb, ok := brigade.(*stream.BucketBrigade)
	if !ok {
		return phpv.ZNULL.ZVal(), nil
	}

	return bb.MakeWriteable(ctx), nil
}

// PhpUserFilterClass is the built-in php_user_filter class that user-defined
// stream filters extend.
var PhpUserFilterClass = &phpobj.ZClass{
	Name: "php_user_filter",
	Props: []*phpv.ZClassProp{
		{VarName: "filtername", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		{VarName: "params", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
		{VarName: "stream", Default: phpv.ZNULL.ZVal(), Modifiers: phpv.ZAttrPublic},
	},
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"filter": {
			Name: "filter",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZInt(stream.PSFS_FEED_ME).ZVal(), nil
			}),
			ReturnType: phpv.ParseTypeHint("int"),
		},
		"oncreate": {
			Name: "onCreate",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZTrue.ZVal(), nil
			}),
			ReturnType: phpv.ParseTypeHint("bool"),
		},
		"onclose": {
			Name: "onClose",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
			ReturnType: phpv.ParseTypeHint("void"),
		},
	},
}

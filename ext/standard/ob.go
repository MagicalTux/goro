package standard

import (
	"errors"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// output buffering functions

// > func bool ob_start ([ callable $output_callback = NULL [, int $chunk_size = 0 [, int $flags = PHP_OUTPUT_HANDLER_STDFLAGS ]]] )
func fncObStart(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	g := ctx.Global().(*phpctx.Global)

	// Check if OB system was disabled by a previous re-entrant fatal error
	if g.IsObDisabled() {
		ctx.Notice("Failed to create buffer")
		return phpv.ZBool(false).ZVal(), nil
	}

	if ctx.GetConfig("ob_in_handler", phpv.ZBool(false).ZVal()).AsBool(ctx) {
		g.SetObDisabled()
		return nil, ctx.Errorf("ob_start(): Cannot use output buffering in output buffering display handlers")
	}

	// Resolve callback manually so we can emit warnings instead of fatal errors.
	// ob_start produces specific warning messages that differ from SpawnCallable errors.
	var callback phpv.Callable
	if len(args) > 0 && args[0] != nil && !args[0].IsNull() {
		cb, err := obResolveCallback(ctx, args[0].Dup())
		if err != nil {
			ctx.Warn("%s", err)
			ctx.Notice("Failed to create buffer")
			return phpv.ZBool(false).ZVal(), nil
		}
		callback = cb
	}

	// Parse remaining args (chunkSize, flags) starting at index 1
	var chunkSize *phpv.ZInt
	var flags *phpv.ZInt
	if len(args) > 1 {
		core.ExpandAt(ctx, args, 1, &chunkSize)
	}
	if len(args) > 2 {
		core.ExpandAt(ctx, args, 2, &flags)
	}

	b := g.AppendBuffer()

	if callback != nil {
		b.CB = callback
	}

	if chunkSize != nil && int(*chunkSize) > 0 {
		b.ChunkSize = int(*chunkSize)
	}

	if flags != nil {
		// Only accept capability flags from user, strip status/type flags
		capabilityMask := phpctx.BufferCleanable | phpctx.BufferFlushable | phpctx.BufferRemovable
		b.Flags = int(*flags) & capabilityMask
	}

	return phpv.ZBool(true).ZVal(), nil
}

// > func void ob_flush ( void )
func fncObFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to flush buffer. No buffer to flush")
		return phpv.ZBool(false).ZVal(), nil
	}
	if !buf.IsFlushable() {
		ctx.Notice("Failed to flush buffer of %s (%d)", buf.CallbackName(), buf.Level())
		return phpv.ZBool(false).ZVal(), nil
	}
	buf.SetCaller(ctx, "ob_flush")
	defer buf.ClearCaller()
	return phpv.ZBool(true).ZVal(), buf.Flush()
}

// > func void ob_clean ( void )
func fncObClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete buffer. No buffer to delete")
		return phpv.ZBool(false).ZVal(), nil
	}
	if !buf.IsCleanable() {
		ctx.Notice("Failed to delete buffer of %s (%d)", buf.CallbackName(), buf.Level())
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.SetCaller(ctx, "ob_clean")
	defer buf.ClearCaller()
	err := buf.Clean()
	return phpv.ZBool(true).ZVal(), err
}

// > func bool ob_end_clean ( void )
func fncObEndClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete buffer. No buffer to delete")
		return phpv.ZBool(false).ZVal(), nil
	}
	if !buf.IsCleanable() || !buf.IsRemovable() {
		ctx.Notice("Failed to discard buffer of %s (%d)", buf.CallbackName(), buf.Level())
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.SetCaller(ctx, "ob_end_clean")
	defer buf.ClearCaller()
	err := buf.CloseClean()
	return phpv.ZBool(true).ZVal(), err
}

// > func bool ob_end_flush ( void )
func fncObEndFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete and flush buffer. No buffer to delete or flush")
		return phpv.ZBool(false).ZVal(), nil
	}
	if !buf.IsFlushable() || !buf.IsRemovable() {
		ctx.Notice("Failed to send buffer of %s (%d)", buf.CallbackName(), buf.Level())
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.SetCaller(ctx, "ob_end_flush")
	defer buf.ClearCaller()
	return phpv.ZBool(true).ZVal(), buf.Close()
}

// > func int ob_get_level ( void )
func fncObGetLevel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	return phpv.ZInt(buf.Level() + 1).ZVal(), nil
}

// > func string ob_get_clean ( void )
func fncObGetClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	if !buf.IsCleanable() || !buf.IsRemovable() {
		if !buf.IsCleanable() {
			ctx.Notice("Failed to discard buffer of %s (%d)", buf.CallbackName(), buf.Level())
		}
		if !buf.IsRemovable() {
			ctx.Notice("Failed to delete buffer of %s (%d)", buf.CallbackName(), buf.Level())
		}
		return data, nil
	}

	buf.SetCaller(ctx, "ob_get_clean")
	defer buf.ClearCaller()
	err := buf.CloseClean()
	return data, err
}

// > func string ob_get_contents ( void )
func fncObGetContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	return data, nil
}

// > func string ob_get_flush ( void )
func fncObGetFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if ctx.GetConfig("ob_in_handler", phpv.ZBool(false).ZVal()).AsBool(ctx) {
		return nil, ctx.Errorf("ob_get_flush(): Cannot use output buffering in output buffering display handlers")
	}

	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	if !buf.IsFlushable() || !buf.IsRemovable() {
		if !buf.IsFlushable() {
			ctx.Notice("Failed to send buffer of %s (%d)", buf.CallbackName(), buf.Level())
		}
		if !buf.IsRemovable() {
			ctx.Notice("Failed to delete buffer of %s (%d)", buf.CallbackName(), buf.Level())
		}
		return data, nil
	}

	buf.SetCaller(ctx, "ob_get_flush")
	defer buf.ClearCaller()
	return data, buf.Close()
}

// > func void ob_implicit_flush ([ int $flag = 1 ] )
func fncObImplicitFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	// ob_implicit_flush only affects the top-level (sapi) output, not user buffers.
	// We set it on the Global's implicit flush flag rather than on any specific buffer.
	g := ctx.Global().(*phpctx.Global)
	g.ImplicitFlush = (v == nil) || (*v != 0)

	return phpv.ZNULL.ZVal(), nil
}

// > func int|false ob_get_length ( void )
func fncObGetLength(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(len(buf.Get())).ZVal(), nil
}

// > func array ob_get_status ([ bool $full_status = false ] )
func fncObGetStatus(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fullStatus *phpv.ZBool
	_, err := core.Expand(ctx, args, &fullStatus)
	if err != nil {
		return nil, err
	}

	g := ctx.Global().(*phpctx.Global)
	buf := g.Buffer()

	if fullStatus != nil && bool(*fullStatus) {
		// Return array of all buffer status, ordered from bottom (level 0) to top
		var buffers []*phpctx.Buffer
		for b := buf; b != nil; b = b.Parent() {
			buffers = append(buffers, b)
		}
		result := phpv.NewZArray()
		for i := len(buffers) - 1; i >= 0; i-- {
			status := bufferStatus(buffers[i])
			result.OffsetSet(ctx, nil, status.ZVal())
		}
		return result.ZVal(), nil
	}

	if buf == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	return bufferStatus(buf).ZVal(), nil
}

func bufferStatus(buf *phpctx.Buffer) *phpv.ZArray {
	status := phpv.NewZArray()
	status.OffsetSet(nil, phpv.ZString("name"), phpv.ZString(buf.CallbackName()).ZVal())
	status.OffsetSet(nil, phpv.ZString("type"), phpv.ZInt(buf.Type()).ZVal())
	status.OffsetSet(nil, phpv.ZString("flags"), phpv.ZInt(buf.StatusFlags()).ZVal())
	status.OffsetSet(nil, phpv.ZString("level"), phpv.ZInt(buf.Level()).ZVal())
	status.OffsetSet(nil, phpv.ZString("chunk_size"), phpv.ZInt(buf.ChunkSize).ZVal())
	// PHP aligns buffer_size to 4096-byte chunks based on chunk_size.
	// chunk_size 0 → 16384 (default), otherwise round up to next 4096 multiple.
	bufSize := 16384
	if buf.ChunkSize > 0 {
		bufSize = ((buf.ChunkSize + 4095) / 4096) * 4096
	}
	status.OffsetSet(nil, phpv.ZString("buffer_size"), phpv.ZInt(bufSize).ZVal())
	status.OffsetSet(nil, phpv.ZString("buffer_used"), phpv.ZInt(len(buf.Get())).ZVal())
	return status
}

// > func array ob_list_handlers ( void )
func fncObListHandlers(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	g := ctx.Global().(*phpctx.Global)
	// Collect all buffers, then output from bottom to top
	var buffers []*phpctx.Buffer
	for b := g.Buffer(); b != nil; b = b.Parent() {
		buffers = append(buffers, b)
	}
	result := phpv.NewZArray()
	for i := len(buffers) - 1; i >= 0; i-- {
		result.OffsetSet(ctx, nil, phpv.ZString(buffers[i].CallbackName()).ZVal())
	}
	return result.ZVal(), nil
}

// obResolveCallback tries to resolve a callable for ob_start, returning a plain
// error (not PhpError/PhpThrow) with ob_start-specific messages on failure.
func obResolveCallback(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error) {
	switch v.GetType() {
	case phpv.ZtString:
		s := string(v.AsString(ctx))
		if idx := strings.Index(s, "::"); idx >= 0 {
			className := s[:idx]
			methodName := s[idx+2:]
			class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), false)
			if err != nil {
				return nil, errors.New("class " + className + " does not have a method \"" + methodName + "\"")
			}
			member, ok := class.GetMethod(phpv.ZString(methodName).ToLower())
			if !ok {
				return nil, errors.New("class " + className + " does not have a method \"" + methodName + "\"")
			}
			if !member.Modifiers.IsStatic() {
				return nil, errors.New("non-static method " + className + "::" + methodName + "() cannot be called statically")
			}
			return phpv.BindClass(member.Method, class, true), nil
		}
		cb, err := ctx.Global().GetFunction(ctx, phpv.ZString(s))
		if err != nil {
			return nil, errors.New("function \"" + s + "\" not found or invalid function name")
		}
		return cb, nil
	case phpv.ZtArray:
		array := v.Array()
		// Check exact count — PHP requires exactly 2 elements for array callbacks
		if ht := v.HashTable(); ht == nil || ht.Count() != 2 {
			return nil, errors.New("array callback must have exactly two members")
		}
		firstArg, err := array.OffsetGet(ctx, phpv.ZInt(0))
		if err != nil {
			return nil, errors.New("array callback must have exactly two members")
		}
		methodName, err := array.OffsetGet(ctx, phpv.ZInt(1))
		if err != nil {
			return nil, errors.New("array callback must have exactly two members")
		}
		var class phpv.ZClass
		var instance phpv.ZObject
		switch firstArg.GetType() {
		case phpv.ZtString:
			class, err = ctx.Global().GetClass(ctx, firstArg.AsString(ctx), false)
			if err != nil {
				return nil, errors.New("class \"" + string(firstArg.AsString(ctx)) + "\" not found")
			}
		case phpv.ZtObject:
			instance = firstArg.AsObject(ctx)
			class = instance.GetClass()
		default:
			return nil, errors.New("array callback must have exactly two members")
		}
		if methodName.GetType() != phpv.ZtString {
			return nil, errors.New("array callback must have exactly two members")
		}
		name := methodName.AsString(ctx).ToLower()
		member, ok := class.GetMethod(name)
		if !ok {
			return nil, errors.New("class " + string(class.GetName()) + " does not have a method \"" + string(methodName.AsString(ctx)) + "\"")
		}
		if instance != nil {
			return phpv.Bind(member.Method, instance), nil
		}
		return phpv.BindClass(member.Method, class, true), nil
	case phpv.ZtObject:
		// Try SpawnCallable for objects (closures, __invoke)
		cb, err := core.SpawnCallable(ctx, v)
		if err != nil {
			return nil, errors.New("no array or string given")
		}
		return cb, nil
	default:
		return nil, errors.New("no array or string given")
	}
}

// > func bool output_add_rewrite_var ( string $name, string $value )
func fncOutputAddRewriteVar(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	var value phpv.ZString
	_, err := core.Expand(ctx, args, &name, &value)
	if err != nil {
		return nil, err
	}

	// Track memory allocation for the name and value. PHP internally
	// copies the strings for the URL rewriting rules, so we track
	// the full allocation including the copies.
	g := ctx.Global().(*phpctx.Global)
	// PHP allocates: name copy + value copy + internal rule structures
	// For large strings this dominates, so track 2x (name + value copies)
	allocSize := uint64(len(name)+len(value)) * 2
	if allocSize > 0 {
		if err := g.MemAlloc(ctx, allocSize); err != nil {
			return nil, ctx.Errorf("Allowed memory size of %d bytes exhausted (tried to allocate %d bytes)",
				g.MemLimit(), allocSize)
		}
	}

	// Store for URL rewriting (stub: accept but don't implement rewriting)
	return phpv.ZBool(true).ZVal(), nil
}

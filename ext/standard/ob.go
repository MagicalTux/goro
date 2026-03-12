package standard

import (
	"errors"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// output buffering functions

// > func bool ob_start ([ callable $output_callback = NULL [, int $chunk_size = 0 [, int $flags = PHP_OUTPUT_HANDLER_STDFLAGS ]]] )
func fncObStart(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var outputCallback *phpv.Callable
	var chunkSize *phpv.ZInt
	var flags *phpv.ZInt
	_, err := core.Expand(ctx, args, &outputCallback, &chunkSize, &flags)
	if err != nil {
		return nil, err
	}

	if ctx.GetConfig("ob_in_handler", phpv.ZBool(false).ZVal()).AsBool(ctx) {
		return nil, errors.New("ob_start(): Cannot use output buffering in output buffering display handlers")
	}

	b := ctx.Global().(*phpctx.Global).AppendBuffer()

	if outputCallback != nil {
		b.CB = *outputCallback
	}

	if chunkSize != nil {
		b.ChunkSize = int(*chunkSize)
	}

	// TODO flags

	return phpv.ZBool(true).ZVal(), nil
}

// > func void ob_flush ( void )
func fncObFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to flush buffer. No buffer to flush")
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), buf.Flush()
}

// > func void ob_clean ( void )
func fncObClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete buffer. No buffer to delete")
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.Clean()
	return phpv.ZBool(true).ZVal(), nil
}

// > func bool ob_end_clean ( void )
func fncObEndClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete buffer. No buffer to delete")
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.Clean()
	return phpv.ZBool(true).ZVal(), buf.Close()
}

// > func bool ob_end_flush ( void )
func fncObEndFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		ctx.Notice("Failed to delete and flush buffer. No buffer to delete or flush")
		return phpv.ZBool(false).ZVal(), nil
	}

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
	buf.Clean()
	err := buf.Close()

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
	buf := ctx.Global().(*phpctx.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	return data, buf.Flush()
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
		// Return array of all buffer status
		result := phpv.NewZArray()
		for b := buf; b != nil; b = b.Parent() {
			status := bufferStatus(b)
			result.OffsetSet(ctx, nil, status.ZVal())
		}
		return result.ZVal(), nil
	}

	if buf == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	return bufferStatus(buf).ZVal(), nil
}

func bufferCallbackName(buf *phpctx.Buffer) string {
	if buf.CB == nil {
		return "default output handler"
	}
	return buf.CB.Name()
}

func bufferStatus(buf *phpctx.Buffer) *phpv.ZArray {
	status := phpv.NewZArray()
	name := bufferCallbackName(buf)
	status.OffsetSet(nil, phpv.ZString("name"), phpv.ZString(name).ZVal())
	status.OffsetSet(nil, phpv.ZString("type"), phpv.ZInt(0).ZVal())
	status.OffsetSet(nil, phpv.ZString("flags"), phpv.ZInt(112).ZVal())
	status.OffsetSet(nil, phpv.ZString("level"), phpv.ZInt(buf.Level()).ZVal())
	status.OffsetSet(nil, phpv.ZString("chunk_size"), phpv.ZInt(buf.ChunkSize).ZVal())
	status.OffsetSet(nil, phpv.ZString("buffer_size"), phpv.ZInt(len(buf.Get())).ZVal())
	status.OffsetSet(nil, phpv.ZString("buffer_used"), phpv.ZInt(len(buf.Get())).ZVal())
	return status
}

// > func array ob_list_handlers ( void )
func fncObListHandlers(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	g := ctx.Global().(*phpctx.Global)
	for b := g.Buffer(); b != nil; b = b.Parent() {
		name := bufferCallbackName(b)
		result.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
	}
	return result.ZVal(), nil
}

package standard

import (
	"errors"

	"github.com/MagicalTux/gophp/core"
)

// output buffering functions

//> func bool ob_start ([ callable $output_callback = NULL [, int $chunk_size = 0 [, int $flags = PHP_OUTPUT_HANDLER_STDFLAGS ]]] )
func fncObStart(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var outputCallback *core.Callable
	var chunkSize *core.ZInt
	var flags *core.ZInt
	_, err := core.Expand(ctx, args, &outputCallback, &chunkSize, &flags)
	if err != nil {
		return nil, err
	}

	if ctx.GetConfig("ob_in_handler", core.ZBool(false).ZVal()).AsBool(ctx) {
		return nil, errors.New("ob_start(): Cannot use output buffering in output buffering display handlers")
	}

	b := ctx.Global().AppendBuffer()

	if outputCallback != nil {
		b.CB = *outputCallback
	}

	if chunkSize != nil {
		b.ChunkSize = int(*chunkSize)
	}

	// TODO flags

	return core.ZBool(true).ZVal(), nil
}

//> func void ob_flush ( void )
func fncObFlush(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf != nil {
		return core.ZNULL.ZVal(), buf.Flush()
	}
	return core.ZNULL.ZVal(), nil
}

//> func void ob_clean ( void )
func fncObClean(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZNULL.ZVal(), nil
	}

	buf.Clean()
	return core.ZNULL.ZVal(), nil
}

//> func bool ob_end_clean ( void )
func fncObEndClean(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZBool(false).ZVal(), nil
	}

	buf.Clean()
	return core.ZBool(true).ZVal(), buf.Close()
}

//> func bool ob_end_flush ( void )
func fncObEndFlush(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZBool(false).ZVal(), nil
	}

	return core.ZBool(true).ZVal(), buf.Close()
}

//> func int ob_get_level ( void )
func fncObGetLevel(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZInt(0).ZVal(), nil
	}

	return core.ZInt(buf.Level()).ZVal(), buf.Close()
}

//> func string ob_get_clean ( void )
func fncObGetClean(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZBool(false).ZVal(), nil
	}

	data := core.ZString(buf.Get()).ZVal()
	buf.Clean()

	return data, nil
}

//> func string ob_get_contents ( void )
func fncObGetContents(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZBool(false).ZVal(), nil
	}

	data := core.ZString(buf.Get()).ZVal()

	return data, nil
}

//> func string ob_get_flush ( void )
func fncObGetFlush(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZBool(false).ZVal(), nil
	}

	data := core.ZString(buf.Get()).ZVal()

	return data, buf.Flush()
}

//> func void ob_implicit_flush ([ int $flag = 1 ] )
func fncObImplicitFlush(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var v *core.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	buf := ctx.Global().Buffer()
	if buf == nil {
		return core.ZNULL.ZVal(), nil
	}

	buf.ImplicitFlush = (v == nil) || (*v != 0)

	return core.ZNULL.ZVal(), nil
}

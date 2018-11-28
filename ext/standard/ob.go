package standard

import (
	"errors"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// output buffering functions

//> func bool ob_start ([ callable $output_callback = NULL [, int $chunk_size = 0 [, int $flags = PHP_OUTPUT_HANDLER_STDFLAGS ]]] )
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

	b := ctx.Global().(*core.Global).AppendBuffer()

	if outputCallback != nil {
		b.CB = *outputCallback
	}

	if chunkSize != nil {
		b.ChunkSize = int(*chunkSize)
	}

	// TODO flags

	return phpv.ZBool(true).ZVal(), nil
}

//> func void ob_flush ( void )
func fncObFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf != nil {
		return phpv.ZNULL.ZVal(), buf.Flush()
	}
	return phpv.ZNULL.ZVal(), nil
}

//> func void ob_clean ( void )
func fncObClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	buf.Clean()
	return phpv.ZNULL.ZVal(), nil
}

//> func bool ob_end_clean ( void )
func fncObEndClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	buf.Clean()
	return phpv.ZBool(true).ZVal(), buf.Close()
}

//> func bool ob_end_flush ( void )
func fncObEndFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), buf.Close()
}

//> func int ob_get_level ( void )
func fncObGetLevel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	return phpv.ZInt(buf.Level()).ZVal(), buf.Close()
}

//> func string ob_get_clean ( void )
func fncObGetClean(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()
	buf.Clean()

	return data, nil
}

//> func string ob_get_contents ( void )
func fncObGetContents(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	return data, nil
}

//> func string ob_get_flush ( void )
func fncObGetFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	data := phpv.ZString(buf.Get()).ZVal()

	return data, buf.Flush()
}

//> func void ob_implicit_flush ([ int $flag = 1 ] )
func fncObImplicitFlush(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v *phpv.ZInt
	_, err := core.Expand(ctx, args, &v)
	if err != nil {
		return nil, err
	}

	buf := ctx.Global().(*core.Global).Buffer()
	if buf == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	buf.ImplicitFlush = (v == nil) || (*v != 0)

	return phpv.ZNULL.ZVal(), nil
}

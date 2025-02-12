package standard

import (
	"bufio"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// > func string printf ( string $format [, mixed $args [, mixed $... ]] )
func fncPrintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &fmt)
	if err != nil {
		return nil, err
	}

	output, err := core.Zprintf(ctx, fmt, args[n:]...)
	if err != nil {
		return output, err
	}

	bytes := []byte(output.String())
	ctx.Write(bytes)

	return phpv.ZInt(len(bytes)).ZVal(), nil
}

// > func string sprintf ( string $format [, mixed $args [, mixed $... ]] )
func fncSprintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &fmt)
	if err != nil {
		return nil, err
	}

	return core.Zprintf(ctx, fmt, args[n:]...)
}

// > func string vsprintf ( string $format [, mixed $args [, mixed $... ]] )
func fncVSprintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fmt phpv.ZString
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(ctx, args, &fmt, &arrayArgs)
	if err != nil {
		return nil, err
	}

	var array []*phpv.ZVal
	iter := arrayArgs.NewIterator()
	for ; iter.Valid(ctx); iter.Next(ctx) {
		val, err := iter.Current(ctx)
		if err != nil {
			return nil, err
		}
		array = append(array, val)
	}

	return core.Zprintf(ctx, fmt, array...)
}

// > func string vprintf ( string $format [, mixed $args [, mixed $... ]] )
func fncVPrintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fmt phpv.ZString
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(ctx, args, &fmt, &arrayArgs)
	if err != nil {
		return nil, err
	}

	var array []*phpv.ZVal
	iter := arrayArgs.NewIterator()
	for ; iter.Valid(ctx); iter.Next(ctx) {
		val, err := iter.Current(ctx)
		if err != nil {
			return nil, err
		}
		array = append(array, val)
	}

	output, err := core.Zprintf(ctx, fmt, array...)
	if err != nil {
		return output, err
	}

	bytes := []byte(output.String())
	ctx.Write(bytes)

	return phpv.ZInt(len(bytes)).ZVal(), nil
}

// > func string fprintf ( resource $handle , string $format [, mixed $... ] )
func fncFPrintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &handle, &fmt)
	if err != nil {
		return nil, err
	}

	var file *stream.Stream
	file, ok := handle.(*stream.Stream)
	if !ok {
		return nil, ctx.Warn("resource not yet supported: %s", handle.String())
	}

	buf := bufio.NewWriter(file)
	defer buf.Flush()

	length, err := core.ZFprintf(ctx, buf, fmt, args[n:]...)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(length).ZVal(), nil
}

// > func int vfprintf ( resource $handle , string $format , array $args )
func fncVFPrintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fmt phpv.ZString
	var arrayArgs *phpv.ZArray
	_, err := core.Expand(ctx, args, &handle, &fmt, &arrayArgs)
	if err != nil {
		return nil, err
	}

	var array []*phpv.ZVal
	for _, val := range arrayArgs.Iterate(ctx) {
		array = append(array, val)
	}

	var file *stream.Stream
	file, ok := handle.(*stream.Stream)
	if !ok {
		return nil, ctx.Warn("resource not yet supported: %s", handle.String())
	}

	buf := bufio.NewWriter(file)
	defer buf.Flush()

	length, err := core.ZFprintf(ctx, buf, fmt, array...)
	if err != nil {
		return nil, err
	}

	return phpv.ZInt(length).ZVal(), nil
}

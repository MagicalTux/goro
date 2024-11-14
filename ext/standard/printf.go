package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
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

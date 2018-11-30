package standard

import (
	"bytes"
	"errors"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

//> func mixed constant ( string $name )
func constant(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	k, ok := ctx.Global().ConstantGet(name)
	if !ok {
		// TODO trigger notice: constant not found
		return phpv.ZNULL.ZVal(), nil
	}
	return k.ZVal(), nil
}

//> func mixed eval ( string $code )
func stdFuncEval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) != 1 {
		return nil, errors.New("eval() requires 1 argument")
	}

	// parse code in args[0]
	z, err := args[0].As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	// tokenize
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte(z.Value().(phpv.ZString))), "-")

	c, err := compiler.Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return c.Run(ctx)
}

//> func mixed hrtime ([ bool $get_as_number = FALSE ] )
func stdFuncHrTime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var getAsNum *bool
	_, err := core.Expand(ctx, args, &getAsNum)
	if err != nil {
		return nil, err
	}

	// TODO find a better time source

	if getAsNum != nil && *getAsNum {
		// do get as num
		return phpv.ZInt(time.Now().UnixNano()).ZVal(), nil
	}

	t := time.Now()
	r := phpv.NewZArray()
	r.OffsetSet(ctx, nil, phpv.ZInt(t.Unix()).ZVal())
	r.OffsetSet(ctx, nil, phpv.ZInt(t.Nanosecond()).ZVal())
	return r.ZVal(), nil
}

//> func int sleep ( int $seconds )
func stdFuncSleep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var t phpv.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Second)

	return phpv.ZInt(0).ZVal(), nil
}

//> func int usleep ( int $seconds )
func stdFuncUsleep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var t phpv.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Microsecond)

	return nil, nil
}

//> func void die ([ string|int $status ] )
func die(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return exit(ctx, args)
}

//> func void exit ([ string|int $status ] )
func exit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ext **phpv.ZVal
	_, err := core.Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext == nil {
		return nil, phpv.ExitError(0)
	}

	z := *ext

	if z.GetType() == phpv.ZtInt {
		return nil, phpv.ExitError(z.AsInt(ctx))
	}

	z, err = z.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	ctx.Write([]byte(z.String()))
	return nil, phpv.ExitError(0)
}

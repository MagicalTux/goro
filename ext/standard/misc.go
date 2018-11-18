package standard

import (
	"bytes"
	"errors"
	"time"

	"github.com/MagicalTux/gophp/core"
	"github.com/MagicalTux/gophp/core/tokenizer"
)

//> func mixed constant ( string $name )
func constant(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var name core.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	return ctx.GetGlobal().GetConstant(name)
}

//> func mixed eval ( string $code )
func stdFuncEval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	if len(args) != 1 {
		return nil, errors.New("eval() requires 1 argument")
	}

	// parse code in args[0]
	z, err := args[0].As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	// tokenize
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte(z.Value().(core.ZString))), "-")

	c, err := core.Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return c.Run(ctx)
}

//> func mixed hrtime ([ bool $get_as_number = FALSE ] )
func stdFuncHrTime(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var getAsNum *bool
	_, err := core.Expand(ctx, args, &getAsNum)
	if err != nil {
		return nil, err
	}

	// TODO find a better time source

	if getAsNum != nil && *getAsNum {
		// do get as num
		return core.ZInt(time.Now().UnixNano()).ZVal(), nil
	}

	t := time.Now()
	r := core.NewZArray()
	r.OffsetSet(ctx, nil, core.ZInt(t.Unix()).ZVal())
	r.OffsetSet(ctx, nil, core.ZInt(t.Nanosecond()).ZVal())
	return r.ZVal(), nil
}

//> func int sleep ( int $seconds )
func stdFuncSleep(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var t core.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Second)

	return core.ZInt(0).ZVal(), nil
}

//> func int usleep ( int $seconds )
func stdFuncUsleep(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var t core.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Microsecond)

	return nil, nil
}

//> func void die ([ string|int $status ] )
func die(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return exit(ctx, args)
}

//> func void exit ([ string|int $status ] )
func exit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var ext **core.ZVal
	_, err := core.Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext == nil {
		return nil, core.ExitError(0)
	}

	z := *ext

	if z.GetType() == core.ZtInt {
		return nil, core.ExitError(z.AsInt(ctx))
	}

	z, err = z.As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	ctx.Write([]byte(z.String()))
	return nil, core.ExitError(0)
}

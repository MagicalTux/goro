package core

import (
	"log"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

//> func mixed include (string filename)
func fncInclude(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx = ctx.Parent(1)
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().(*Global).Include(ctx, fn)
}

func (c *Global) Include(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().(*Global).Open(fn, true)
	if err != nil {
		// include → do not fail if file could not be open (TODO issue warning)
		return nil, nil
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = phpv.ZString(fn2)
	}
	c.included[fn] = true

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return CatchReturn(code.Run(ctx))
}

//> func mixed require (string filename)
func fncRequire(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx = ctx.Parent(1)
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().(*Global).Require(ctx, fn)
}

func (c *Global) Require(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().(*Global).Open(fn, true)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = phpv.ZString(fn2)
	}
	c.included[fn] = true

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	log.Printf("about to run: %s", debugDump(code))

	return CatchReturn(code.Run(ctx))
}

//> func mixed include_once (string filename)
func fncIncludeOnce(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx = ctx.Parent(1)
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().(*Global).IncludeOnce(ctx, fn)
}

func (c *Global) IncludeOnce(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().(*Global).Open(fn, true)
	if err != nil {
		// include → do not fail if file could not be open (TODO issue warning)
		return nil, nil
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = phpv.ZString(fn2)
	}

	if _, ok := c.included[fn]; ok {
		// do not include file
		return nil, nil
	}
	c.included[fn] = true

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return CatchReturn(code.Run(ctx))
}

//> func mixed require_once (string filename)
func fncRequireOnce(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx = ctx.Parent(1)
	var fn phpv.ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().(*Global).RequireOnce(ctx, fn)
}

func (c *Global) RequireOnce(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().(*Global).Open(fn, true)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = phpv.ZString(fn2)
	}
	if _, ok := c.included[fn]; ok {
		// do not include file
		return nil, nil
	}
	c.included[fn] = true

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return CatchReturn(code.Run(ctx))
}

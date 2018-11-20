package core

import "github.com/MagicalTux/gophp/core/tokenizer"

//> func mixed include (string filename)
func fncInclude(ctx Context, args []*ZVal) (*ZVal, error) {
	var fn ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().Include(ctx, fn)
}

func (c *Global) Include(ctx Context, fn ZString) (*ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
	if err != nil {
		// include → do not fail if file could not be open (TODO issue warning)
		return nil, nil
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = ZString(fn2)
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
func fncRequire(ctx Context, args []*ZVal) (*ZVal, error) {
	var fn ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().Require(ctx, fn)
}

func (c *Global) Require(ctx Context, fn ZString) (*ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = ZString(fn2)
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

//> func mixed include_once (string filename)
func fncIncludeOnce(ctx Context, args []*ZVal) (*ZVal, error) {
	var fn ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().IncludeOnce(ctx, fn)
}

func (c *Global) IncludeOnce(ctx Context, fn ZString) (*ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
	if err != nil {
		// include → do not fail if file could not be open (TODO issue warning)
		return nil, nil
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = ZString(fn2)
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
func fncRequireOnce(ctx Context, args []*ZVal) (*ZVal, error) {
	var fn ZString
	_, err := Expand(ctx, args, &fn)
	if err != nil {
		return nil, err
	}

	return ctx.Global().RequireOnce(ctx, fn)
}

func (c *Global) RequireOnce(ctx Context, fn ZString) (*ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = ZString(fn2)
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

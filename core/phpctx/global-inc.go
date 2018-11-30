package phpctx

import (
	"log"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

var Compile func(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error)

func (c *Global) Include(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
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

	return phperr.CatchReturn(code.Run(ctx))
}

func (c *Global) Require(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
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

	log.Printf("about to run: %s", phpv.DebugDump(code))

	return phperr.CatchReturn(code.Run(ctx))
}

func (c *Global) IncludeOnce(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
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

	return phperr.CatchReturn(code.Run(ctx))
}

func (c *Global) RequireOnce(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := ctx.Global().Open(fn, true)
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

	return phperr.CatchReturn(code.Run(ctx))
}

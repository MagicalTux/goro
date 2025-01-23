package phpctx

import (
	"errors"
	"os"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type includeContext struct {
	phpv.Context
	scriptFilename phpv.ZString
}

func (ic *includeContext) GetScriptFile() phpv.ZString {
	return ic.scriptFilename
}

var Compile func(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error)

func (c *Global) DoString(ctx phpv.Context, strCode phpv.ZString) (*phpv.ZVal, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}

	_, err = f.WriteString(`<?php ` + string(strCode))
	if err != nil {
		return nil, err
	}

	f.Sync()
	f.Seek(0, 0)
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	// tokenize
	t := tokenizer.NewLexer(f, string(f.Name()))

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(code.Run(ctx))
}

func (c *Global) Include(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := c.openForInclusion(ctx, fn)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, c.FuncError(err)
	}

	if f == nil {
		return nil, ctx.Warn(
			"%s(%s): failed to open stream: No such file or directory",
			ctx.GetFuncName(),
			string(fn),
			logopt.NoFuncName(true),
		)
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = phpv.ZString(fn2)
	}
	c.included[fn] = true

	ctx = &includeContext{
		Context:        ctx,
		scriptFilename: fn,
	}

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
	f, err := c.openForInclusion(ctx, fn)
	if err != nil {
		return nil, c.FuncError(err)
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

func (c *Global) IncludeOnce(ctx phpv.Context, fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := c.openForInclusion(ctx, fn)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, c.FuncError(err)
	}

	if f == nil {
		return nil, ctx.Warn(
			"%s(%s): failed to open stream: No such file or directory",
			ctx.GetFuncName(),
			string(fn),
			logopt.NoFuncName(true),
		)
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
	f, err := c.openForInclusion(ctx, fn)
	if err != nil {
		return nil, c.FuncError(err)
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

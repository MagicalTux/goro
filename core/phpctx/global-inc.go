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
	varScope       phpv.Context // scope for variable lookups (caller's context)
}

func (ic *includeContext) GetScriptFile() phpv.ZString {
	return ic.scriptFilename
}

// Parent returns the embedded Context (the include's FuncContext) so that
// the stack trace walker can find it when traversing the context chain.
func (ic *includeContext) Parent(n int) phpv.Context {
	if n <= 1 {
		return ic.Context
	}
	return ic.Context.Parent(n - 1)
}

// Variable operations delegate to the caller's scope (varScope), not the
// include's FuncContext. This preserves PHP semantics where included files
// share the caller's variable scope, while keeping the include's FuncContext
// in the parent chain for stack traces.
func (ic *includeContext) OffsetExists(ctx phpv.Context, name phpv.Val) (bool, error) {
	return ic.varScope.OffsetExists(ctx, name)
}
func (ic *includeContext) OffsetGet(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, error) {
	return ic.varScope.OffsetGet(ctx, name)
}
func (ic *includeContext) OffsetCheck(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, bool, error) {
	return ic.varScope.OffsetCheck(ctx, name)
}
func (ic *includeContext) OffsetSet(ctx phpv.Context, name phpv.Val, v *phpv.ZVal) error {
	return ic.varScope.OffsetSet(ctx, name, v)
}
func (ic *includeContext) OffsetUnset(ctx phpv.Context, name phpv.Val) error {
	return ic.varScope.OffsetUnset(ctx, name)
}
func (ic *includeContext) Count(ctx phpv.Context) phpv.ZInt {
	return ic.varScope.Count(ctx)
}
func (ic *includeContext) NewIterator() phpv.ZIterator {
	return ic.varScope.NewIterator()
}

type globalContextNoID struct {
	phpv.GlobalContext
}

func (g *globalContextNoID) Global() phpv.GlobalContext { return g }

func (g *globalContextNoID) NextResourceID() int { return 0 }

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
	defer t.Close()

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
		// Capture loc before the first warning — the error handler callback
		// may change the global loc pointer, causing the second warning to
		// report the wrong line.
		warnOpt := logopt.Data{NoFuncName: true, Loc: ctx.Loc()}
		if err := ctx.Warn(
			"include(%s): Failed to open stream: No such file or directory",
			string(fn),
			warnOpt,
		); err != nil {
			return nil, err
		}
		includePath := ctx.GetConfig("include_path", phpv.ZStr("."))
		return nil, ctx.Warn(
			"include(): Failed opening '%s' for inclusion (include_path='%s')",
			string(fn),
			includePath.String(),
			warnOpt,
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
		varScope:       ctx.Parent(1),
	}

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))
	defer t.Close()

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(code.Run(ctx))
}

// Similar to Require() but disables allocating resource ID
// for the first opened main script.
func (c *Global) requireMain(fn phpv.ZString) (*phpv.ZVal, error) {
	f, err := c.openForInclusion(&globalContextNoID{c}, fn)
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
	defer t.Close()

	// compile
	code, err := Compile(c, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(code.Run(c))
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

	ctx = &includeContext{
		Context:        ctx,
		scriptFilename: fn,
		varScope:       ctx.Parent(1),
	}

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))
	defer t.Close()

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
		warnOpt := logopt.Data{NoFuncName: true, Loc: ctx.Loc()}
		if err := ctx.Warn(
			"include_once(%s): Failed to open stream: No such file or directory",
			string(fn),
			warnOpt,
		); err != nil {
			return nil, err
		}
		includePath := ctx.GetConfig("include_path", phpv.ZStr("."))
		return nil, ctx.Warn(
			"include_once(): Failed opening '%s' for inclusion (include_path='%s')",
			string(fn),
			includePath.String(),
			warnOpt,
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

	ctx = &includeContext{
		Context:        ctx,
		scriptFilename: fn,
		varScope:       ctx.Parent(1),
	}

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))
	defer t.Close()

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

	ctx = &includeContext{
		Context:        ctx,
		scriptFilename: fn,
		varScope:       ctx.Parent(1),
	}

	// tokenize
	t := tokenizer.NewLexer(f, string(fn))
	defer t.Close()

	// compile
	code, err := Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(code.Run(ctx))
}

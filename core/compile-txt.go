package core

import "github.com/MagicalTux/gophp/core/tokenizer"

type runInlineHtml string

func compileInlineHtml(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	return runInlineHtml(i.Data), nil
}

func (s runInlineHtml) Run(ctx Context) (*ZVal, error) {
	_, err := ctx.Write([]byte(s))
	return nil, err
}

func (s runInlineHtml) Loc() *Loc {
	return nil
}

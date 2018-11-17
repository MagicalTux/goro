package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

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

func (s runInlineHtml) Dump(w io.Writer) error {
	_, err := w.Write([]byte("\n?>\n"))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("<?php\n"))
	return err
}

package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runInlineHtml string

func compileInlineHtml(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	return runInlineHtml(i.Data), nil
}

func (s runInlineHtml) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	_, err := ctx.Write([]byte(s))
	return nil, err
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

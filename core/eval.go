package core

import (
	"bytes"
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func Eval(ctx phpv.Context, expr string) (*phpv.ZVal, error) {
	if strings.TrimSpace(expr) == "" {
		expr = `""`
	}
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte("return "+expr+";")), "-")
	c, err := compiler.Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(c.Run(ctx))
}

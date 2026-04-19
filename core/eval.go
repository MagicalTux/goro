package core

import (
	"bytes"
	"strings"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

func Eval(ctx phpv.Context, expr string) (*phpv.ZVal, error) {
	if strings.TrimSpace(expr) == "" {
		expr = `""`
	}
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte("return "+expr+";")), "-")
	defer t.Close()
	c, err := compiler.Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return phperr.CatchReturn(c.Run(ctx))
}

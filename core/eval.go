package core

import (
	"bytes"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// type Evaluator func(ctx phpv.Context, expr string) (*phpv.ZVal, error)
func Eval(ctx phpv.Context, expr string) (*phpv.ZVal, error) {
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte(expr)), "-")
	// TODO: export compile expr, or make expressions a valid PHP statement
	c, err := compiler.Compile(ctx, t)
	if err != nil {
		return nil, err
	}

	return c.Run(ctx)
}

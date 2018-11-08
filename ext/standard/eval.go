package standard

import (
	"bytes"
	"errors"

	"git.atonline.com/tristantech/gophp/core"
	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

func stdFuncEval(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	if len(args) != 1 {
		return nil, errors.New("eval() requires 1 argument")
	}

	// parse code in args[0]
	z, err := args[0].As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	// tokenize
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte(z.Value().(core.ZString))), "-")

	c := core.Compile(ctx, t)

	return c.Run(ctx)
}

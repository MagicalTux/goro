package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

// an expression is:

// $a_variable
// "a string"
// "a string with a $var"
// $a + $b

type runVariable string

func (r runVariable) run(ctx Context) (*ZVal, error) {
	return &ZVal{ZString("TODO:" + r)}, nil
}

func compileExpr(c *compileCtx) (runnable, error) {
	for {
		var v runnable

		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_VARIABLE:
			v = runVariable(i.Data[1:])
		default:
			return nil, i.Unexpected()
		}

		return v, errors.New("todo expr")
	}
}

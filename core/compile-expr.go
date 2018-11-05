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

	i, err = c.NextItem()
	if err != nil {
		return v, err
	}

	// can be any kind of glue (operators, etc)
	switch i.Type {
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '+', '-', '/', '*': // TODO list
			// what follows is also an expression
			t_v, err := compileExpr(c)
			if err != nil {
				return nil, err
			}

			// TODO: math priority
			return &runOperator{op: i.Data, a: v, b: t_v}, nil
		}
	case tokenizer.T_AND_EQUAL, tokenizer.T_BOOLEAN_AND, tokenizer.T_BOOLEAN_OR, tokenizer.T_CONCAT_EQUAL, tokenizer.T_DIV_EQUAL: // etc... FIXME TODO
		// what follows is also an expression
		t_v, err := compileExpr(c)
		if err != nil {
			return nil, err
		}

		// TODO math priority
		return &runOperator{op: i.Data, a: v, b: t_v}, nil
	}

	return v, i.Unexpected()
}

type runOperator struct {
	op string

	a, b runnable
}

func (r *runOperator) run(ctx Context) (*ZVal, error) {
	// TODO
	return nil, errors.New("todo")
}

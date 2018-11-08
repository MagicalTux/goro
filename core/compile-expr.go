package core

import (
	"strconv"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

// an expression is:

// $a_variable
// "a string"
// "a string with a $var"
// $a + $b

type runVariable string

func (r runVariable) Run(ctx Context) (*ZVal, error) {
	return ctx.GetVariable(ZString(r))
}

func (r runVariable) WriteValue(ctx Context, value *ZVal) error {
	return ctx.SetVariable(ZString(r), value)
}

func compileExpr(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var v Runnable
	var err error
	var is_operator bool

	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	switch i.Type {
	case tokenizer.T_VARIABLE:
		v = runVariable(i.Data[1:])
	case tokenizer.T_LNUMBER:
		v, err := strconv.ParseInt(i.Data, 0, 64)
		return &ZVal{ZInt(v)}, err
	case tokenizer.T_DNUMBER:
		v, err := strconv.ParseFloat(i.Data, 64)
		return &ZVal{ZFloat(v)}, err
	case tokenizer.T_STRING:
		// if next is '(' this is a function call
		t_next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		switch t_next.Type {
		case tokenizer.ItemSingleChar:
			switch []rune(t_next.Data)[0] {
			case '(':
				args, err := compileFuncPassedArgs(c)
				if err != nil {
					return nil, err
				}
				v = &runnableFunctionCall{ZString(i.Data), args}
			}
		}
	case tokenizer.T_CONSTANT_ENCAPSED_STRING:
		v, err = compileQuoteConstant(i, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_START_HEREDOC:
		v, err = compileQuoteHeredoc(i, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '"':
			v, err = compileQuoteEncapsed(i, c)
			if err != nil {
				return nil, err
			}
		case '!':
			// this is an operator
			v = nil
			is_operator = true
		case '@':
			// this is a silent operator
			// TODO: we should encase result from compileExpr into a "silencer"
			return compileExpr(nil, c)
		default:
			return nil, i.Unexpected()
		}
	default:
		h, ok := itemTypeHandler[i.Type]
		if ok && h != nil {
			v, err = h.f(i, c)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, i.Unexpected()
		}
	}

	// load operator, if any
	if !is_operator {
		i, err = c.NextItem()
		if err != nil {
			return v, err
		}
	}

	// can be any kind of glue (operators, etc)
	switch i.Type {
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '+', '-', '/', '*', '=', '.', '<', '>', '!': // TODO list
			// what follows is also an expression
			t_v, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}

			// TODO: math priority
			return &runOperator{op: i.Data, a: v, b: t_v}, nil
		case '(':
			// this is a function call of whatever is before
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			return &runnableFunctionCallRef{v, args}, nil
		case ';':
			c.backup()
			// just a value
			return v, nil
		}
	case tokenizer.T_AND_EQUAL, tokenizer.T_BOOLEAN_AND, tokenizer.T_BOOLEAN_OR, tokenizer.T_CONCAT_EQUAL, tokenizer.T_DIV_EQUAL, tokenizer.T_IS_EQUAL, tokenizer.T_MINUS_EQUAL: // etc... FIXME TODO
		// what follows is also an expression
		t_v, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// TODO math priority
		return &runOperator{op: i.Data, a: v, b: t_v}, nil
	}

	// unknown?
	c.backup()
	return v, nil
}

package core

import (
	"errors"
	"path"
	"strconv"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

// an expression is:

// $a_variable
// "a string"
// "a string with a $var"
// $a + $b

type runVariable struct {
	v ZString
	l *Loc
}

func (r *runVariable) Run(ctx Context) (*ZVal, error) {
	return ctx.GetVariable(r.v)
}

func (r *runVariable) WriteValue(ctx Context, value *ZVal) error {
	return ctx.SetVariable(r.v, value)
}

func (r *runVariable) Loc() *Loc {
	return r.l
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

	l := MakeLoc(i.Loc())

	switch i.Type {
	case tokenizer.T_VARIABLE:
		v = &runVariable{ZString(i.Data[1:]), MakeLoc(i.Loc())}
	case tokenizer.T_LNUMBER:
		v, err := strconv.ParseInt(i.Data, 0, 64)
		return &runZVal{ZInt(v), MakeLoc(i.Loc())}, err
	case tokenizer.T_DNUMBER:
		v, err := strconv.ParseFloat(i.Data, 64)
		return &runZVal{ZFloat(v), MakeLoc(i.Loc())}, err
	case tokenizer.T_STRING:
		// if next is '(' this is a function call
		t_next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		gotSomething := false
		switch t_next.Type {
		case tokenizer.T_PAAMAYIM_NEKUDOTAYIM:
			// this is a static method class
			// if v is "parent", "static" or "self" it might not actually be a static call
			return nil, errors.New("todo class static call")
		case tokenizer.ItemSingleChar:
			switch []rune(t_next.Data)[0] {
			case '(':
				args, err := compileFuncPassedArgs(c)
				if err != nil {
					return nil, err
				}
				v = &runnableFunctionCall{ZString(i.Data), args, l}
				gotSomething = true
			}
		}
		if !gotSomething {
			// it's a constant
			v = &runConstant{i.Data, l}
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
	case tokenizer.T_ARRAY:
		v, err = compileArray(i, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_FILE:
		v = &runZVal{ZString(l.Filename), l}
	case tokenizer.T_LINE:
		v = &runZVal{ZInt(l.Line), l}
	case tokenizer.T_DIR:
		v = &runZVal{ZString(path.Dir(l.Filename)), l}
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '"':
			v, err = compileQuoteEncapsed(i, c, '"')
			if err != nil {
				return nil, err
			}
		case '`':
			v, err = compileQuoteEncapsed(i, c, '`')
			if err != nil {
				return nil, err
			}
			v = &runnableFunctionCall{"shell_exec", []Runnable{v}, l}
		case '!':
			// this is an operator
			v = nil
			is_operator = true
		case '@':
			// this is a silent operator
			// TODO: we should encase result from compileExpr into a "silencer"
			v, err = compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
		case '[':
			v, err = compileArray(i, c)
			if err != nil {
				return nil, err
			}
		case '(':
			// sub-expr
			v, err = compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if !i.IsSingle(')') {
				return nil, i.Unexpected()
			}
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

	return compilePostExpr(v, i, c)
}

func compilePostExpr(v Runnable, i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	l := MakeLoc(i.Loc())
	// can be any kind of glue (operators, etc)
	switch i.Type {
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '+', '-', '/', '*', '=', '.', '<', '>', '!', '|', '^', '&': // TODO list
			// what follows is also an expression
			t_v, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}

			// TODO: math priority
			return &runOperator{op: i.Data, a: v, b: t_v, l: l}, nil
		case '?':
			return compileTernaryOp(v, c)
		case '(':
			// this is a function call of whatever is before
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			return &runnableFunctionCallRef{v, args, l}, nil
		case '[':
			c.backup()
			return compileArrayAccess(v, c)
		case ';':
			c.backup()
			// just a value
			return v, nil
		}
	case tokenizer.T_AND_EQUAL, tokenizer.T_BOOLEAN_AND, tokenizer.T_BOOLEAN_OR, tokenizer.T_CONCAT_EQUAL, tokenizer.T_DIV_EQUAL, tokenizer.T_IS_EQUAL, tokenizer.T_IS_NOT_EQUAL, tokenizer.T_MINUS_EQUAL: // etc... FIXME TODO
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

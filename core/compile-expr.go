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

	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	l := MakeLoc(i.Loc())

	switch i.Type {
	case tokenizer.T_VARIABLE:
		v = &runVariable{ZString(i.Data[1:]), l}
		return compilePostExpr(v, nil, c)
	case tokenizer.T_LNUMBER:
		v, err := strconv.ParseInt(i.Data, 0, 64)
		if err == nil {
			return compilePostExpr(&runZVal{ZInt(v), l}, nil, c)
		}
		// if ParseInt failed, try to parse as float (value too large?)
		fallthrough
	case tokenizer.T_DNUMBER:
		v, err := strconv.ParseFloat(i.Data, 64)
		if err != nil {
			errv := err.(*strconv.NumError)
			if errv.Err == strconv.ErrRange {
				// v is inf
				return compilePostExpr(&runZVal{ZFloat(v), l}, nil, c)
			}
			return nil, err
		}
		return compilePostExpr(&runZVal{ZFloat(v), l}, nil, c)
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
			// this is a static method call or a static variable access

			// nb: if i.Data is "parent", "static" or "self" it might not actually be a static call
			switch i.Data {
			case "parent", "static", "self":
				return nil, errors.New("todo class special call")
			default:
				className := ZString(i.Data)
				c.NextItem()          // T_PAAMAYIM_NEKUDOTAYIM
				i, err = c.NextItem() // actual value, a T_VARIABLE (if var) or a T_STRING

				switch i.Type {
				case tokenizer.T_VARIABLE:
					v = &runClassStaticVarRef{className, ZString(i.Data[1:]), l}
					return compilePostExpr(v, nil, c)
				case tokenizer.T_STRING:
					v = &runClassStaticObjRef{className, ZString(i.Data), l}
					return compilePostExpr(v, nil, c)
				default:
					return nil, i.Unexpected()
				}
			}
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
	case tokenizer.T_BOOL_CAST, tokenizer.T_INT_CAST, tokenizer.T_ARRAY_CAST, tokenizer.T_DOUBLE_CAST, tokenizer.T_OBJECT_CAST, tokenizer.T_STRING_CAST:
		// perform a cast operation on the following (note: v is null)
		t_v, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return spawnRunCast(i.Type, t_v, l)
	case tokenizer.T_INC:
		t_v, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return &runIncDec{inc: true, v: t_v, l: l, post: false}, nil
	case tokenizer.T_DEC:
		t_v, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return &runIncDec{inc: false, v: t_v, l: l, post: false}, nil
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
			// this is an operator, let compilePostExpr() deal with it
			return compilePostExpr(nil, i, c)
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

	return compilePostExpr(v, nil, c)
}

func compilePostExpr(v Runnable, i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	if i == nil {
		var err error
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}
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
	case tokenizer.T_INC:
		// v followed by inc
		return &runIncDec{inc: true, v: v, l: l, post: true}, nil
	case tokenizer.T_DEC:
		// v followed by dec
		return &runIncDec{inc: false, v: v, l: l, post: true}, nil
	case tokenizer.T_AND_EQUAL,
		tokenizer.T_BOOLEAN_AND,
		tokenizer.T_BOOLEAN_OR,
		tokenizer.T_CONCAT_EQUAL,
		tokenizer.T_PLUS_EQUAL,
		tokenizer.T_MINUS_EQUAL,
		tokenizer.T_MUL_EQUAL,
		tokenizer.T_DIV_EQUAL,
		tokenizer.T_IS_EQUAL,
		tokenizer.T_IS_NOT_EQUAL,
		tokenizer.T_IS_IDENTICAL,
		tokenizer.T_IS_NOT_IDENTICAL,
		tokenizer.T_LOGICAL_AND,
		tokenizer.T_LOGICAL_XOR,
		tokenizer.T_LOGICAL_OR: // etc... FIXME TODO

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

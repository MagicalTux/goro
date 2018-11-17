package core

import (
	"errors"
	"path"
	"strconv"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

// an expression is:

// $a_variable
// "a string"
// "a string with a $var"
// $a + $b
// etc...

func compileExpr(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	res, err := compileOneExpr(i, c)
	if err != nil {
		return nil, err
	}

	for {
		sr, err := compilePostExpr(res, nil, c)
		if err != nil {
			return nil, err
		}
		if sr == nil {
			return res, nil
		}
		res = sr
	}
}

func compileOneExpr(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// fetch only one expression, without any operator or anything
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
		return &runVariable{ZString(i.Data[1:]), l}, nil
	case tokenizer.T_LNUMBER:
		v, err := strconv.ParseInt(i.Data, 0, 64)
		if err == nil {
			return &runZVal{ZInt(v), l}, nil
		}
		// if ParseInt failed, try to parse as float (value too large?)
		fallthrough
	case tokenizer.T_DNUMBER:
		v, err := strconv.ParseFloat(i.Data, 64)
		if err != nil {
			errv := err.(*strconv.NumError)
			if errv.Err == strconv.ErrRange {
				// v is inf
				return &runZVal{ZFloat(v), l}, nil
			}
			return nil, err
		}
		return &runZVal{ZFloat(v), l}, nil
	case tokenizer.T_STRING:
		// if next is '(' this is a function call
		t_next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()

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
					return &runClassStaticVarRef{className, ZString(i.Data[1:]), l}, nil
				case tokenizer.T_STRING:
					return &runClassStaticObjRef{className, ZString(i.Data), l}, nil
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
				return &runnableFunctionCall{ZString(i.Data), args, l}, nil
			}
		}
		// so it's a constant
		return &runConstant{i.Data, l}, nil
	case tokenizer.T_CONSTANT_ENCAPSED_STRING:
		return compileQuoteConstant(i, c)
	case tokenizer.T_START_HEREDOC:
		return compileQuoteHeredoc(i, c)
	case tokenizer.T_ARRAY:
		return compileArray(i, c)
	case tokenizer.T_FILE:
		return &runZVal{ZString(l.Filename), l}, nil
	case tokenizer.T_LINE:
		return &runZVal{ZInt(l.Line), l}, nil
	case tokenizer.T_DIR:
		return &runZVal{ZString(path.Dir(l.Filename)), l}, nil
	case tokenizer.T_BOOL_CAST, tokenizer.T_INT_CAST, tokenizer.T_ARRAY_CAST, tokenizer.T_DOUBLE_CAST, tokenizer.T_OBJECT_CAST, tokenizer.T_STRING_CAST:
		// perform a cast operation on the following (note: v is null)
		//TODO make this an operator for appropriate operator precedence
		t_v, err := compileOneExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return spawnRunCast(i.Type, t_v, l)
	case tokenizer.T_INC:
		t_v, err := compileOneExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return &runIncDec{inc: true, v: t_v, l: l, post: false}, nil
	case tokenizer.T_DEC:
		t_v, err := compileOneExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return &runIncDec{inc: false, v: t_v, l: l, post: false}, nil
	case tokenizer.ItemSingleChar:
		ch := []rune(i.Data)[0]
		switch ch {
		case '"':
			return compileQuoteEncapsed(i, c, '"')
		case '`':
			v, err := compileQuoteEncapsed(i, c, '`')
			if err != nil {
				return nil, err
			}
			return &runnableFunctionCall{"shell_exec", []Runnable{v}, l}, nil
		case '!', '+', '-', '~':
			// this is an operator, let compilePostExpr() deal with it
			return compilePostExpr(nil, i, c)
		case '@':
			// this is a silent operator
			// TODO: we should encase result from compileExpr into a "silencer"
			return compileOneExpr(nil, c)
		case '[':
			return compileArray(i, c)
		case '(':
			// sub-expr
			v, err := compileExpr(nil, c)
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
			return v, err
		case '&':
			// get ref of something
			v, err := compileOneExpr(nil, c)
			if err != nil {
				return nil, err
			}

			return compilePostExpr(&runRef{v, l}, nil, c)
		default:
			return nil, i.Unexpected()
		}
	default:
		h, ok := itemTypeHandler[i.Type]
		if ok && h != nil {
			return h.f(i, c)
		} else {
			return nil, i.Unexpected()
		}
	}
	return nil, i.Unexpected()
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
		case '+', '-', '/', '*', '=', '.', '<', '>', '!', '|', '^', '&', '%', '~': // TODO list
			// what follows is also an expression
			t_v, err := compileOneExpr(nil, c)
			if err != nil {
				return nil, err
			}

			// TODO: math priority
			return spawnOperator(i.Data, v, t_v, l)
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
		case '[', '{':
			c.backup()
			return compileArrayAccess(v, c)
		case ';':
			c.backup()
			// just a value
			return nil, nil
		}
	case tokenizer.T_INC:
		// v followed by inc
		return compilePostExpr(&runIncDec{inc: true, v: v, l: l, post: true}, nil, c)
	case tokenizer.T_DEC:
		// v followed by dec
		return compilePostExpr(&runIncDec{inc: false, v: v, l: l, post: true}, nil, c)
	case tokenizer.T_OBJECT_OPERATOR:
		return compileObjectOperator(v, i, c)
	case tokenizer.T_AND_EQUAL,
		tokenizer.T_POW,
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
		tokenizer.T_IS_GREATER_OR_EQUAL,
		tokenizer.T_IS_SMALLER_OR_EQUAL,
		tokenizer.T_LOGICAL_AND,
		tokenizer.T_LOGICAL_XOR,
		tokenizer.T_SL,
		tokenizer.T_SR,
		tokenizer.T_SL_EQUAL,
		tokenizer.T_SR_EQUAL,
		tokenizer.T_LOGICAL_OR: // etc... FIXME TODO

		// what follows is also an expression
		t_v, err := compileOneExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// TODO math priority
		return spawnOperator(i.Data, v, t_v, l)
	}

	// unknown?
	c.backup()
	return nil, nil
}

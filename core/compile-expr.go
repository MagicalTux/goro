package core

import (
	"errors"
	"fmt"
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

func compileExpr(i *tokenizer.Item, c compileCtx) (Runnable, error) {
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

func compileOpExpr(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	res, err := compileOneExpr(i, c)
	if err != nil {
		return nil, err
	}

	for {
		if isOperator(c.peekType()) {
			return res, nil
		}
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

func compileOneExpr(i *tokenizer.Item, c compileCtx) (Runnable, error) {
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
	case tokenizer.Rune('$'):
		return compileRunVariableRef(nil, c, l)
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
		case tokenizer.Rune('('):
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			return &runnableFunctionCall{ZString(i.Data), args, l}, nil
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
	case tokenizer.T_CLASS:
		class := c.getClass()
		if class == nil {
			return nil, errors.New("__CLASS__ outside of a class")
		}
		return &runZVal{class.Name, l}, nil
	case tokenizer.T_METHOD_C:
		class := c.getClass()
		f := c.getFunc()
		if class == nil || f == nil {
			return &runZVal{ZString(""), l}, nil
		}

		return &runZVal{ZString(fmt.Sprintf("%s::%s", class.Name, f.name)), l}, nil
	case tokenizer.T_BOOL_CAST, tokenizer.T_INT_CAST, tokenizer.T_ARRAY_CAST, tokenizer.T_DOUBLE_CAST, tokenizer.T_OBJECT_CAST, tokenizer.T_STRING_CAST:
		// perform a cast operation on the following (note: v is null)
		// make this an operator for appropriate operator precedence
		t_v, err := compileOpExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return spawnOperator(i.Type, nil, t_v, l)
	case tokenizer.T_INC, tokenizer.T_DEC:
		// this is an operator, let compilePostExpr() deal with it
		return compilePostExpr(nil, i, c)
	case tokenizer.Rune('"'):
		return compileQuoteEncapsed(i, c, '"')
	case tokenizer.Rune('`'):
		v, err := compileQuoteEncapsed(i, c, '`')
		if err != nil {
			return nil, err
		}
		return &runnableFunctionCall{"shell_exec", []Runnable{v}, l}, nil
	case tokenizer.Rune('!'), tokenizer.Rune('+'), tokenizer.Rune('-'), tokenizer.Rune('~'), tokenizer.Rune('@'):
		// this is an operator, let compilePostExpr() deal with it
		return compilePostExpr(nil, i, c)
	case tokenizer.Rune('['):
		return compileArray(i, c)
	case tokenizer.Rune('('):
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
		// put the expr into a container to avoid
		return &runParentheses{v}, err
	case tokenizer.Rune('&'):
		// get ref of something
		// TODO make this operator?
		v, err := compileOpExpr(nil, c)
		if err != nil {
			return nil, err
		}

		return &runRef{v, l}, nil
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

func compilePostExpr(v Runnable, i *tokenizer.Item, c compileCtx) (Runnable, error) {
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
	case tokenizer.Rune('?'):
		return compileTernaryOp(v, c)
	case tokenizer.Rune('('):
		// this is a function call of whatever is before
		c.backup()
		args, err := compileFuncPassedArgs(c)
		if err != nil {
			return nil, err
		}
		return &runnableFunctionCallRef{v, args, l}, nil
	case tokenizer.Rune('['), tokenizer.Rune('{'):
		c.backup()
		return compileArrayAccess(v, c)
	case tokenizer.Rune(';'):
		c.backup()
		// just a value
		return nil, nil
	case tokenizer.T_INC, tokenizer.T_DEC:
		if v == nil {
			// what follows is also an expression
			t_v, err := compileOpExpr(nil, c)
			if err != nil {
				return nil, err
			}
			return spawnOperator(i.Type, nil, t_v, l)
		} else {
			return spawnOperator(i.Type, v, nil, l)
		}
	case tokenizer.T_OBJECT_OPERATOR:
		return compileObjectOperator(v, i, c)
	default:
		if isOperator(i.Type) {
			// what follows should be an expression
			t_v, err := compileOpExpr(nil, c)
			if err != nil {
				return nil, err
			}

			return spawnOperator(i.Type, v, t_v, l)
		}
	}

	// unknown?
	c.backup()
	return nil, nil
}

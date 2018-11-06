package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type funcArg struct {
	varName      string
	required     bool
	defaultValue runnable
}

type runnableFunction struct {
	name string
	args []*funcArg
	code runnable
}

type runnableFunctionCall struct {
	name string
	args []runnable
}

func (r *runnableFunction) run(ctx Context) (l *ZVal, err error) {
	// TODO: create new variables local context, set collected arguments, and run
	return r.code.run(ctx)
}

func (r *runnableFunctionCall) run(ctx Context) (l *ZVal, err error) {
	return nil, errors.New("todo")
}

func compileFunction(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	// typically T_FUNCTION is followed by:
	// - a name and parameters → this is a regular function
	// - directly parameters → this is a lambda function

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	switch i.Type {
	case tokenizer.T_STRING:
		// regular function definition
		return compileFunctionWithName(i.Data, c)
	case tokenizer.ItemSingleChar:
		if i.Data == "(" {
			// function with no name is lambda
			c.backup()
			return compileFunctionWithName("", c)
		}
	}

	return nil, i.Unexpected()
}

func compileSpecialFuncCall(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	// special function call that comes without (), so as a keyword. Example: echo, die, etc
	has_open := false
	fn_name := i.Data

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle('(') {
		has_open = true
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(')') {
			return &runnableFunctionCall{fn_name, nil}, nil
		}
		if i.IsSingle(';') {
			c.backup()
			return &runnableFunctionCall{fn_name, nil}, nil
		}
	}

	var args []runnable

	// parse passed arguments
	for {
		var a runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		args = append(args, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}
		if has_open && i.IsSingle(')') {
			return &runnableFunctionCall{fn_name, args}, nil
		}
		if !has_open && i.IsSingle(';') {
			c.backup()
			return &runnableFunctionCall{fn_name, args}, nil
		}

		return nil, i.Unexpected()
	}
}

func compileFunctionWithName(name string, c *compileCtx) (runnable, error) {
	var err error
	args, err := compileFunctionArgs(c)

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	_ = args
	body, err := compileBase(nil, c)
	if err != nil {
		return nil, err
	}

	return &runnableFunction{
		name: name,
		args: args,
		code: body,
	}, nil
}

func compileFunctionArgs(c *compileCtx) (res []*funcArg, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse arguments
	for {
		// in a function delcaration, we must have a T_VARIABLE now
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		arg := &funcArg{}
		arg.varName = i.Data[1:] // skip $
		arg.required = true      // typically

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		if !i.IsSingle('=') {
			return nil, i.Unexpected()
		}

		// what follows is an expression, a default value of sorts
		return nil, errors.New("function arg default value TODO") // TODO FIXME
	}
}

func compileFuncPassedArgs(c *compileCtx) (res []runnable, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse passed arguments
	for {
		var a runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		res = append(res, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}

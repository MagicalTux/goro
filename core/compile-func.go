package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type runnableFunction struct {
	name    ZString
	use     []ZString
	closure *ZClosure
	l       *Loc
}

type runnableFunctionCall struct {
	name ZString
	args []Runnable
	l    *Loc
}

type runnableFunctionCallRef struct {
	name Runnable
	args []Runnable
	l    *Loc
}

func (r *runnableFunction) Loc() *Loc {
	return r.l
}

func (r *runnableFunctionCall) Loc() *Loc {
	return r.l
}

func (r *runnableFunctionCallRef) Loc() *Loc {
	return r.l
}

func (r *runnableFunction) Run(ctx Context) (l *ZVal, err error) {
	c := r.closure.dup()
	// collect use vars
	for _, s := range r.use {
		z, err := ctx.GetVariable(s)
		if err != nil {
			return nil, err
		}
		c.use = append(c.use, &funcUse{s, z})
	}
	return &ZVal{c}, nil
}

func (r *runnableFunctionCall) Run(ctx Context) (l *ZVal, err error) {
	// grab function
	f, err := ctx.GetFunction(r.name)
	if err != nil {
		return nil, err
	}
	// collect args
	f_arg := make([]*ZVal, len(r.args))
	for i, a := range r.args {
		f_arg[i], err = a.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	return f.Call(ctx, f_arg)
}

func (r *runnableFunctionCallRef) Run(ctx Context) (l *ZVal, err error) {
	v, err := r.name.Run(ctx)
	if err != nil {
		return nil, err
	}

	var f Callable
	var ok bool

	if f, ok = v.v.(Callable); !ok {
		v, err = v.As(ctx, ZtString)
		if err != nil {
			return nil, err
		}
		// grab function
		f, err = ctx.GetFunction(v.v.(ZString))
		if err != nil {
			return nil, err
		}
	}

	// collect args
	f_arg := make([]*ZVal, len(r.args))
	for i, a := range r.args {
		f_arg[i], err = a.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	return f.Call(ctx, f_arg)
}

func compileFunction(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// typically T_FUNCTION is followed by:
	// - a name and parameters → this is a regular function
	// - directly parameters → this is a lambda function
	l := MakeLoc(i.Loc())

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	switch i.Type {
	case tokenizer.T_STRING:
		// regular function definition
		return compileFunctionWithName(ZString(i.Data), c, l)
	case tokenizer.ItemSingleChar:
		if i.Data == "(" {
			// function with no name is lambda
			c.backup()
			return compileFunctionWithName("", c, l)
		}
	}

	return nil, i.Unexpected()
}

func compileSpecialFuncCall(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// special function call that comes without (), so as a keyword. Example: echo, die, etc
	has_open := false
	fn_name := ZString(i.Data)
	l := MakeLoc(i.Loc())

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
			return &runnableFunctionCall{fn_name, nil, l}, nil
		}
		if i.IsSingle(';') {
			c.backup()
			return &runnableFunctionCall{fn_name, nil, l}, nil
		}
	}

	var args []Runnable

	// parse passed arguments
	for {
		var a Runnable
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
			return &runnableFunctionCall{fn_name, args, l}, nil
		}
		if !has_open && i.IsSingle(';') {
			c.backup()
			return &runnableFunctionCall{fn_name, args, l}, nil
		}

		return nil, i.Unexpected()
	}
}

func compileFunctionWithName(name ZString, c *compileCtx, l *Loc) (Runnable, error) {
	var err error
	var use []ZString
	args, err := compileFunctionArgs(c)

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_USE && name == "" {
		// anonymous function variables
		use, err = compileFunctionUse(c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	body, err := compileBase(nil, c)
	if err != nil {
		return nil, err
	}

	return &runnableFunction{
		name: name,
		use:  use,
		closure: &ZClosure{
			args:  args,
			code:  body,
			start: l,
			// TODO populate end
		},
		l: l,
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
		arg.varName = ZString(i.Data[1:]) // skip $
		arg.required = true               // typically

		res = append(res, arg)

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

func compileFunctionUse(c *compileCtx) (res []ZString, err error) {
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

		res = append(res, ZString(i.Data[1:])) // skip $

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

func compileFuncPassedArgs(c *compileCtx) (res []Runnable, err error) {
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
		var a Runnable
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
